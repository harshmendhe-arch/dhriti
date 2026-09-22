package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/opencode-ai/dhriti/internal/llm/tools"
	"github.com/opencode-ai/dhriti/internal/logging"
	"github.com/opencode-ai/dhriti/internal/message"
	"google.golang.org/genai"
)

const (
	defaultGeminiStreamModel = "gemini-3.1-flash-live-preview"
	geminiLiveURL            = "wss://generativelanguage.googleapis.com/ws/google.ai.generativelanguage.v1beta.GenerativeService.BidiGenerateContent"
)

// stream connects to the Gemini Live API over WebSocket and drives the event
// channel until turnComplete, error, or context cancellation.
func (g *geminiClient) stream(ctx context.Context, messages []message.Message, tools []tools.BaseTool) <-chan ProviderEvent {
	eventChan := make(chan ProviderEvent)
	go func() {
		defer close(eventChan)
		g.streamLive(ctx, messages, tools, eventChan)
	}()
	return eventChan
}

func (g *geminiClient) streamLive(ctx context.Context, messages []message.Message, toolList []tools.BaseTool, eventChan chan<- ProviderEvent) {
	streamModel := g.providerOptions.model.StreamModel
	if streamModel == "" {
		streamModel = defaultGeminiStreamModel
	}

	attempts := 0
	for {
		attempts++
		err := g.runLiveSession(ctx, streamModel, messages, toolList, eventChan)
		if err == nil {
			return
		}
		if ctx.Err() != nil {
			eventChan <- ProviderEvent{Type: EventError, Error: ctx.Err()}
			return
		}
		retry, after, retryErr := g.shouldRetry(attempts, err)
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

func (g *geminiClient) runLiveSession(ctx context.Context, streamModel string, messages []message.Message, toolList []tools.BaseTool, eventChan chan<- ProviderEvent) error {
	u := geminiLiveURL + "?key=" + url.QueryEscape(g.providerOptions.apiKey)
	header := http.Header{}

	conn, err := dialWebSocket(ctx, u, header)
	if err != nil {
		return err
	}
	defer closeWebSocket(conn)

	if err := g.liveSetup(conn, streamModel, toolList); err != nil {
		return err
	}

	if err := g.liveSeedHistory(conn, messages); err != nil {
		return err
	}

	return g.liveReadLoop(ctx, conn, eventChan)
}

func (g *geminiClient) liveSetup(conn *websocket.Conn, streamModel string, toolList []tools.BaseTool) error {
	setup := map[string]any{
		"setup": map[string]any{
			"model": "models/" + streamModel,
			"generationConfig": map[string]any{
				"maxOutputTokens":     g.providerOptions.maxTokens,
				"responseModalities":  []string{"TEXT"},
			},
			"systemInstruction": map[string]any{
				"parts": []map[string]any{{"text": g.providerOptions.systemMessage}},
			},
		},
	}

	if len(toolList) > 0 {
		decls := make([]map[string]any, 0, len(toolList))
		for _, tool := range toolList {
			info := tool.Info()
			decls = append(decls, map[string]any{
				"name":        info.Name,
				"description": info.Description,
				"parameters": map[string]any{
					"type":       "object",
					"properties": info.Parameters,
					"required":   info.Required,
				},
			})
		}
		setup["setup"].(map[string]any)["tools"] = []map[string]any{
			{"functionDeclarations": decls},
		}
	}

	if err := conn.WriteJSON(setup); err != nil {
		return err
	}

	// Wait for setupComplete
	for {
		_, raw, err := conn.ReadMessage()
		if err != nil {
			return err
		}
		var msg map[string]any
		if err := json.Unmarshal(raw, &msg); err != nil {
			continue
		}
		if _, ok := msg["setupComplete"]; ok {
			return nil
		}
		if errPayload, ok := msg["error"]; ok {
			return fmt.Errorf("gemini live setup error: %v", errPayload)
		}
	}
}

func (g *geminiClient) liveSeedHistory(conn *websocket.Conn, messages []message.Message) error {
	// Build turns from convertMessages output (full history including the last user message).
	turns := g.convertMessages(messages)
	if len(turns) == 0 {
		return nil
	}

	payload := map[string]any{
		"clientContent": map[string]any{
			"turns":        turns,
			"turnComplete": true,
		},
	}
	return conn.WriteJSON(payload)
}

func (g *geminiClient) liveReadLoop(ctx context.Context, conn *websocket.Conn, eventChan chan<- ProviderEvent) error {
	currentContent := ""
	toolCalls := make([]message.ToolCall, 0)
	var finalUsage *genai.UsageMetadata

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
			ServerContent *struct {
				ModelTurn *struct {
					Parts []struct {
						Text         string                 `json:"text"`
						FunctionCall map[string]interface{} `json:"functionCall"`
					} `json:"parts"`
				} `json:"modelTurn"`
				TurnComplete bool `json:"turnComplete"`
				Interrupted  bool `json:"interrupted"`
			} `json:"serverContent"`
			ToolCall *struct {
				FunctionCalls []struct {
					Name string                 `json:"name"`
					Args map[string]interface{} `json:"args"`
					ID   string                 `json:"id"`
				} `json:"functionCalls"`
			} `json:"toolCall"`
			ToolCallCancel *struct{} `json:"toolCallCancel"`
			Error *struct {
				Code    int    `json:"code"`
				Message string `json:"message"`
				Status  string `json:"status"`
			} `json:"error"`
			UsageMetadata *genai.UsageMetadata `json:"usageMetadata"`
			SetupComplete *struct{}            `json:"setupComplete"`
		}

		if err := json.Unmarshal(raw, &envelope); err != nil {
			continue
		}

		if envelope.Error != nil {
			return fmt.Errorf("gemini live error: %s (%s)", envelope.Error.Message, envelope.Error.Status)
		}

		if envelope.UsageMetadata != nil {
			finalUsage = envelope.UsageMetadata
		}

		if envelope.ToolCall != nil {
			for _, fc := range envelope.ToolCall.FunctionCalls {
				id := fc.ID
				if id == "" {
					id = "call_" + uuid.New().String()
				}
				args, _ := json.Marshal(fc.Args)
				newCall := message.ToolCall{
					ID:       id,
					Name:     fc.Name,
					Input:    string(args),
					Type:     "function",
					Finished: true,
				}
				if !containsToolCall(toolCalls, newCall) {
					toolCalls = append(toolCalls, newCall)
				}
			}
		}

		if envelope.ServerContent != nil {
			sc := envelope.ServerContent

			if sc.ModelTurn != nil {
				for _, part := range sc.ModelTurn.Parts {
					if part.Text != "" {
						eventChan <- ProviderEvent{Type: EventContentDelta, Content: part.Text}
						currentContent += part.Text
					}
					if part.FunctionCall != nil {
						id := "call_" + uuid.New().String()
						name, _ := part.FunctionCall["name"].(string)
						args, _ := json.Marshal(part.FunctionCall["args"])
						newCall := message.ToolCall{
							ID:       id,
							Name:     name,
							Input:    string(args),
							Type:     "function",
							Finished: true,
						}
						if !containsToolCall(toolCalls, newCall) {
							toolCalls = append(toolCalls, newCall)
						}
					}
				}
			}

			if sc.TurnComplete || sc.Interrupted {
				finishReason := message.FinishReasonEndTurn
				if len(toolCalls) > 0 {
					finishReason = message.FinishReasonToolUse
				}

				usage := TokenUsage{}
				if finalUsage != nil {
					usage = TokenUsage{
						InputTokens:         int64(finalUsage.PromptTokenCount),
						OutputTokens:        int64(finalUsage.ResponseTokenCount),
						CacheCreationTokens: 0,
						CacheReadTokens:     int64(finalUsage.CachedContentTokenCount),
					}
				}

				eventChan <- ProviderEvent{Type: EventContentStop}
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
			}
		}
	}
}

func containsToolCall(calls []message.ToolCall, candidate message.ToolCall) bool {
	for _, existing := range calls {
		if existing.Name == candidate.Name && existing.Input == candidate.Input {
			return true
		}
	}
	return false
}
