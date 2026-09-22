package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/websocket"
	"github.com/opencode-ai/dhriti/internal/llm/tools"
	"github.com/opencode-ai/dhriti/internal/logging"
	"github.com/opencode-ai/dhriti/internal/message"
)

const (
	defaultOpenAIStreamModel = "gpt-realtime"
	openAIRealtimeURL        = "wss://api.openai.com/v1/realtime"
)

// realtimeEndpoint resolves the WebSocket URL, auth key, and model for a
// Realtime session — gateway first, then official OpenAI defaults.
func (o *openaiClient) realtimeEndpoint() (url, apiKey, model string) {
	if o.providerOptions.gateway.enabled() {
		url = o.providerOptions.gateway.url
		apiKey = o.providerOptions.gateway.apiKey
		model = o.providerOptions.gateway.model
		if model == "" {
			model = o.providerOptions.model.StreamModel
		}
		if model == "" {
			model = defaultOpenAIStreamModel
		}
		return
	}

	url = openAIRealtimeURL
	apiKey = o.providerOptions.apiKey
	model = o.providerOptions.model.StreamModel
	if model == "" {
		model = defaultOpenAIStreamModel
	}
	return
}

// stream connects to the OpenAI Realtime API over WebSocket and drives the
// event channel until response.done, error, or context cancellation.
func (o *openaiClient) stream(ctx context.Context, messages []message.Message, tools []tools.BaseTool) <-chan ProviderEvent {
	eventChan := make(chan ProviderEvent)
	go func() {
		defer close(eventChan)
		o.streamRealtime(ctx, messages, tools, eventChan)
	}()
	return eventChan
}

func (o *openaiClient) streamRealtime(ctx context.Context, messages []message.Message, toolList []tools.BaseTool, eventChan chan<- ProviderEvent) {
	attempts := 0
	for {
		attempts++
		err := o.runRealtimeSession(ctx, messages, toolList, eventChan, &attempts)
		if err == nil {
			return
		}
		if ctx.Err() != nil {
			eventChan <- ProviderEvent{Type: EventError, Error: ctx.Err()}
			return
		}
		retry, after, retryErr := o.shouldRetry(attempts, err)
		if retryErr != nil {
			eventChan <- ProviderEvent{Type: EventError, Error: retryErr}
			return
		}
		if !retry {
			eventChan <- ProviderEvent{Type: EventError, Error: err}
			return
		}
		logging.WarnPersist(fmt.Sprintf("Retrying due to rate limit... attempt %d of %d", attempts, maxRetries), logging.PersistTimeArg, time.Millisecond*time.Duration(after+100))
		select {
		case <-ctx.Done():
			eventChan <- ProviderEvent{Type: EventError, Error: ctx.Err()}
			return
		case <-time.After(time.Duration(after) * time.Millisecond):
			continue
		}
	}
}

func (o *openaiClient) runRealtimeSession(ctx context.Context, messages []message.Message, toolList []tools.BaseTool, eventChan chan<- ProviderEvent, attempts *int) error {
	endpointURL, apiKey, model := o.realtimeEndpoint()
	url := fmt.Sprintf("%s?model=%s", endpointURL, model)
	header := http.Header{}
	if apiKey != "" {
		header.Set("Authorization", "Bearer "+apiKey)
	}
	for k, v := range o.options.extraHeaders {
		header.Set(k, v)
	}

	conn, err := dialWebSocket(ctx, url, header)
	if err != nil {
		return err
	}
	defer closeWebSocket(conn)

	if err := o.realtimeHandshake(conn, toolList); err != nil {
		return err
	}

	if err := o.realtimeReplayHistory(conn, messages, eventChan); err != nil {
		return err
	}

	if err := conn.WriteJSON(map[string]any{"type": "response.create"}); err != nil {
		return err
	}

	return o.realtimeReadLoop(ctx, conn, eventChan, attempts)
}

// sendViaGateway runs a one-shot Realtime session over the gateway WebSocket
// and returns the completed ProviderResponse (used by non-streaming agents).
func (o *openaiClient) sendViaGateway(ctx context.Context, messages []message.Message, toolList []tools.BaseTool) (*ProviderResponse, error) {
	events := o.stream(ctx, messages, toolList)
	for ev := range events {
		switch ev.Type {
		case EventComplete:
			if ev.Response != nil {
				return ev.Response, nil
			}
			return &ProviderResponse{}, nil
		case EventError:
			if ev.Error != nil {
				return nil, ev.Error
			}
			return nil, fmt.Errorf("gateway stream error")
		case EventWarning:
			// non-fatal; continue
		}
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return nil, fmt.Errorf("gateway stream ended without complete event")
}

func (o *openaiClient) realtimeHandshake(conn *websocket.Conn, toolList []tools.BaseTool) error {
	// Wait for session.created
	for {
		_, raw, err := conn.ReadMessage()
		if err != nil {
			return err
		}
		var msg struct {
			Type string `json:"type"`
		}
		if err := json.Unmarshal(raw, &msg); err != nil {
			continue
		}
		if msg.Type == "session.created" {
			break
		}
		if msg.Type == "error" {
			return fmt.Errorf("realtime handshake error: %s", string(raw))
		}
	}

	session := map[string]any{
		"type":              "realtime",
		"instructions":      o.providerOptions.systemMessage,
		"output_modalities": []string{"text"},
		"tool_choice":       "auto",
	}
	if len(toolList) > 0 {
		session["tools"] = o.realtimeTools(toolList)
	}

	return conn.WriteJSON(map[string]any{
		"type":    "session.update",
		"session": session,
	})
}

func (o *openaiClient) realtimeTools(toolList []tools.BaseTool) []map[string]any {
	result := make([]map[string]any, 0, len(toolList))
	for _, tool := range toolList {
		info := tool.Info()
		result = append(result, map[string]any{
			"type":        "function",
			"name":        info.Name,
			"description": info.Description,
			"parameters": map[string]any{
				"type":       "object",
				"properties": info.Parameters,
				"required":   info.Required,
			},
		})
	}
	return result
}

func (o *openaiClient) realtimeReplayHistory(conn *websocket.Conn, messages []message.Message, eventChan chan<- ProviderEvent) error {
	warnedAttachments := false
	for _, msg := range messages {
		switch msg.Role {
		case message.User:
			if len(msg.BinaryContent()) > 0 && !warnedAttachments {
				eventChan <- ProviderEvent{Type: EventWarning, Error: fmt.Errorf("realtime API does not support image attachments; they were dropped")}
				warnedAttachments = true
			}
			if err := conn.WriteJSON(map[string]any{
				"type": "conversation.item.create",
				"item": map[string]any{
					"type":    "message",
					"role":    "user",
					"content": []map[string]any{{"type": "input_text", "text": msg.Content().String()}},
				},
			}); err != nil {
				return err
			}
		case message.Assistant:
			if msg.Content().String() != "" {
				if err := conn.WriteJSON(map[string]any{
					"type": "conversation.item.create",
					"item": map[string]any{
						"type":    "message",
						"role":    "assistant",
						"content": []map[string]any{{"type": "output_text", "text": msg.Content().String()}},
					},
				}); err != nil {
					return err
				}
			}
			for _, call := range msg.ToolCalls() {
				if err := conn.WriteJSON(map[string]any{
					"type": "conversation.item.create",
					"item": map[string]any{
						"type":      "function_call",
						"call_id":   call.ID,
						"name":      call.Name,
						"arguments": call.Input,
					},
				}); err != nil {
					return err
				}
			}
		case message.Tool:
			for _, result := range msg.ToolResults() {
				if err := conn.WriteJSON(map[string]any{
					"type": "conversation.item.create",
					"item": map[string]any{
						"type":    "function_call_output",
						"call_id": result.ToolCallID,
						"output":  result.Content,
					},
				}); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func (o *openaiClient) realtimeReadLoop(ctx context.Context, conn *websocket.Conn, eventChan chan<- ProviderEvent, attempts *int) error {
	currentContent := ""
	toolCalls := make([]message.ToolCall, 0)
	contentStarted := false

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		_, raw, err := conn.ReadMessage()
		if err != nil {
			return err
		}

		var envelope struct {
			Type string          `json:"type"`
			Raw  json.RawMessage `json:"-"`
		}
		if err := json.Unmarshal(raw, &envelope); err != nil {
			continue
		}

		switch envelope.Type {
		case "response.output_text.delta", "response.text.delta":
			var delta struct {
				Delta string `json:"delta"`
			}
			if err := json.Unmarshal(raw, &delta); err == nil && delta.Delta != "" {
				if !contentStarted {
					eventChan <- ProviderEvent{Type: EventContentStart}
					contentStarted = true
				}
				eventChan <- ProviderEvent{Type: EventContentDelta, Content: delta.Delta}
				currentContent += delta.Delta
			}

		case "response.output_item.done":
			var item struct {
				Item struct {
					Type      string `json:"type"`
					CallID    string `json:"call_id"`
					Name      string `json:"name"`
					Arguments string `json:"arguments"`
				} `json:"item"`
			}
			if err := json.Unmarshal(raw, &item); err == nil && item.Item.Type == "function_call" {
				toolCalls = append(toolCalls, message.ToolCall{
					ID:       item.Item.CallID,
					Name:     item.Item.Name,
					Input:    item.Item.Arguments,
					Type:     "function",
					Finished: true,
				})
			}

		case "response.done":
			var done struct {
				Response struct {
					Status         string `json:"status"`
					Usage          *struct {
						InputTokens        int64 `json:"input_tokens"`
						OutputTokens       int64 `json:"output_tokens"`
						InputTokenDetails *struct {
							CachedTokens int64 `json:"cached_tokens"`
						} `json:"input_token_details"`
					} `json:"usage"`
					StatusDetails *struct {
						Reason string `json:"reason"`
					} `json:"status_details"`
				} `json:"response"`
			}
			if err := json.Unmarshal(raw, &done); err != nil {
				return err
			}

			usage := TokenUsage{}
			if done.Response.Usage != nil {
				cached := int64(0)
				if done.Response.Usage.InputTokenDetails != nil {
					cached = done.Response.Usage.InputTokenDetails.CachedTokens
				}
				usage = TokenUsage{
					InputTokens:         done.Response.Usage.InputTokens - cached,
					OutputTokens:        done.Response.Usage.OutputTokens,
					CacheCreationTokens: 0,
					CacheReadTokens:     cached,
				}
			}

			finishReason := message.FinishReasonEndTurn
			switch done.Response.Status {
			case "incomplete":
				if done.Response.StatusDetails != nil && strings.Contains(done.Response.StatusDetails.Reason, "max_output_tokens") {
					finishReason = message.FinishReasonMaxTokens
				} else {
					finishReason = message.FinishReasonUnknown
				}
			case "failed":
				finishReason = message.FinishReasonUnknown
			}
			if len(toolCalls) > 0 {
				finishReason = message.FinishReasonToolUse
			}

			eventChan <- ProviderEvent{
				Type: EventComplete,
				Response: &ProviderResponse{
					Content:      currentContent,
					ToolCalls:    toolCalls,
					Usage:        usage,
					FinishReason: finishReason,
				},
			}
			return nil

		case "error":
			var errPayload struct {
				Error *struct {
					Type    string `json:"type"`
					Code    string `json:"code"`
					Message string `json:"message"`
				} `json:"error"`
			}
			_ = json.Unmarshal(raw, &errPayload)
			msg := "realtime stream error"
			code := ""
			if errPayload.Error != nil {
				msg = errPayload.Error.Message
				code = errPayload.Error.Code
			}
			full := fmt.Sprintf("%s %s", code, msg)
			return fmt.Errorf("%s", full)

		case "response.created", "response.content_part.added", "response.content_part.done",
			"response.output_item.added", "response.output_text.done",
			"response.function_call_arguments.delta", "response.function_call_arguments.done",
			"conversation.item.created", "conversation.item.input_audio_transcription.completed",
			"session.created", "session.updated", "input_audio_buffer.committed",
			"input_audio_buffer.speech_started", "input_audio_buffer.speech_stopped":
			// ignored event types

		default:
			// ignore unknown event types for forward compatibility
		}
	}
}
