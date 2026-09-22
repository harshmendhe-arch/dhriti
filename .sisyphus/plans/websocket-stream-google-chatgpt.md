# Plan: WebSocket-only streaming for Google + ChatGPT; remove all other providers; rename OpenCode → Dhriti

## Goal

1. Keep `send()` (non-streaming: title/summarizer agents) on the existing HTTP SDKs.
2. Replace `stream()` (coder/task agents) for **Gemini** and **OpenAI** with raw WebSocket clients:
   - Gemini → **Live API** (`BidiGenerateContent` WebSocket)
   - OpenAI → **Realtime API** (`wss://api.openai.com/v1/realtime`)
3. Delete every other provider (Anthropic, Bedrock, Copilot, Groq, Azure, VertexAI, OpenRouter, xAI, Local).
4. Rename the product **OpenCode → Dhriti** (full scope: CLI, display name, config paths, env vars, Go module path, docs).

## Key constraint discovered

`stream()` and `send()` share one `model.APIModel`, but the WebSocket endpoints reject chat models:

| Provider | send() model (HTTP) | stream() must use (WS) |
|---|---|---|
| OpenAI | `gpt-4.1`, `o3`, … | `gpt-realtime` (Realtime API only accepts realtime models) |
| Gemini | `gemini-2.5-pro-preview-…` | a Live model, e.g. `gemini-3.1-flash-live-preview` / `gemini-2.5-flash-native-audio-preview-12-2025` |

**Decision:** add an optional `StreamModel` field to `models.Model` (populated for OpenAI + Gemini entries; provider files fall back to a hardcoded default when empty). `send()` keeps using `APIModel`; `stream()` uses `StreamModel`.

## Phase 1 — Prune to OpenAI + Gemini only

### Delete files
- `internal/llm/provider/`: `anthropic.go`, `azure.go`, `bedrock.go`, `copilot.go`, `vertexai.go`
- `internal/llm/models/`: `anthropic.go`, `azure.go`, `copilot.go`, `groq.go`, `local.go`, `openrouter.go`, `vertexai.go`, `xai.go`

### Edit `internal/llm/provider/provider.go`
- `NewProvider` switch: keep only `ProviderOpenAI`, `ProviderGemini` (keep `ProviderMock` panic case).
- `providerClientOptions`: drop `anthropicOptions`, `bedrockOptions`, `copilotOptions`.
- Delete `WithAnthropicOptions`, `WithBedrockOptions`, `WithCopilotOptions`.

### Edit `internal/llm/models/models.go`
- Delete `BedrockClaude37Sonnet` const, `ProviderBedrock` const, the `BedrockClaude37Sonnet` entry in `SupportedModels` (keep `ProviderMock`).
- `ProviderPopularity`: only `ProviderOpenAI`, `ProviderGemini`.
- `init()`: copy only `OpenAIModels` + `GeminiModels`.

### Edit `internal/config/config.go`
- `setProviderDefaults()`: keep only `OPENAI_API_KEY` / `GEMINI_API_KEY`; default-model priority: OpenAI → Gemini (delete all other branches).
- `setDefaultModelForAgent()`: keep only OpenAI + Gemini env-key branches.
- `getProviderAPIKey()`: only `ProviderOpenAI`, `ProviderGemini` cases.
- Delete `hasAWSCredentials`, `hasVertexAICredentials`, `hasCopilotCredentials`, `LoadGitHubToken` (only Copilot used them) and their call sites (incl. GITHUB_TOKEN default at ~line 280).
- Line ~566: reasoning-effort guard → `if model.CanReason && provider == models.ProviderOpenAI`.

### Edit `internal/llm/agent/agent.go` (`createAgentProvider`, ~706–758)
- Delete the Anthropic branch (~741–748); simplify reason guard to `model.Provider == models.ProviderOpenAI && model.CanReason`.

### Edit `cmd/schema/main.go`
- `knownProviders` → `["openai", "gemini"]`.

### Edit `internal/llm/prompt/coder.go` (minimal)
- Default prompt already falls through to `baseAnthropicCoderPrompt` for Gemini; rename nothing — only ensure no removed-provider constants are referenced (currently only `ProviderOpenAI` — no change needed).

### Dependencies
- `go mod tidy` (drops `anthropic-sdk-go`, AWS SDK, Azure identity, etc.; promotes `github.com/gorilla/websocket` to direct once Phase 2/3 imports it).

## Phase 2 — OpenAI `stream()` → Realtime WebSocket

New file `internal/llm/provider/openai_realtime.go` implementing `(o *openaiClient) stream(...)`; delete the SSE loop from `openai.go` (keep `send()`, converters, `shouldRetry` adapted).

### Connection
- URL: `wss://api.openai.com/v1/realtime?model=<streamModel>` (default `gpt-realtime`).
- Header: `Authorization: Bearer <apiKey>` (GA — no beta header).
- Transport: `gorilla/websocket` dialer with custom headers.

### Sequence (one connection per `stream()` call)
1. Wait for `session.created`.
2. `session.update` → `{type:"session.update", session:{type:"realtime", instructions:<systemMessage>, output_modalities:["text"], tools:[{type:"function", name, description, parameters}], tool_choice:"auto"}}`.
3. Replay history via `conversation.item.create`:
   - user → `role:"user"`, `content:[{type:"input_text", …}]`
   - assistant text → `role:"assistant"`, `content:[{type:"output_text", …}]`
   - assistant tool calls → `{type:"function_call", call_id, name, arguments}`
   - tool results → `{type:"function_call_output", call_id, output}`
   - images: Realtime has no image input → emit `EventWarning`, drop attachments.
4. `response.create`.
5. Read loop until `response.done` / `error` / ctx cancel; then close socket.

### Event mapping (preserve existing contract consumed by `agent.processEvent`)
- `response.output_text.delta` (and beta alias `response.text.delta`) → `EventContentDelta`.
- `response.output_item.done` with `type:"function_call"` → accumulate `message.ToolCall{ID: call_id, Name, Input: arguments, Finished: true}`; batch into `EventComplete.Response.ToolCalls` (same as today — agent only reads tool calls from `EventComplete`).
- `response.done` → usage `{input_tokens, output_tokens, input_token_details.cached_tokens}` → `EventComplete` with `FinishReason` from `status` (`completed`→EndTurn, `incomplete`+max_output_tokens→MaxTokens, else Unknown).
- Socket/HTTP errors containing rate-limit signals → reuse backoff+reconnect (adapt `shouldRetry` to plain errors since `openai.Error` type won't wrap WS failures).
- `ctx.Done()` → close conn, emit `EventError{context.Canceled}` only if not already completed.

## Phase 3 — Gemini `stream()` → Live API WebSocket

New file `internal/llm/provider/gemini_live.go` implementing `(g *geminiClient) stream(...)`; delete the `SendMessageStream` loop from `gemini.go` (keep `send()`, converters, `usage`, schema helpers).

### Connection
- URL: `wss://generativelanguage.googleapis.com/ws/google.ai.generativelanguage.v1beta.GenerativeService.BidiGenerateContent?key=<apiKey>`
- streamModel default: `gemini-3.1-flash-live-preview` (fallback constant when `StreamModel` empty; setup sends `models/<id>`).

### Sequence
1. First frame `setup`:
   ```
   {setup:{model:"models/<streamModel>",
     generationConfig:{maxOutputTokens, responseModalities:["TEXT"]},
     systemInstruction:{parts:[{text: systemMessage}]},
     tools:[{functionDeclarations:[…]}]}}
   ```
   Tools reuse the existing `convertTools` shape (`functionDeclarations` with `parameters`).
2. Wait `setupComplete`.
3. Seed history: one `clientContent` frame with `turns` built from existing `convertMessages` output (JSON-marshal `[]*genai.Content` — reuse the converter) and `turnComplete:true`.
4. Read loop until `serverContent.turnComplete` / `toolCall` handling done / error / ctx cancel.

### Event mapping
- `serverContent.modelTurn.parts[].text` → `EventContentDelta`.
- `serverContent` with `functionCall` parts **or** top-level `toolCall.functionCalls[]` → accumulate `message.ToolCall{ID:"call_"+uuid, Name, Input: json(args), Finished:true}` (dedupe like current code); batch into `EventComplete`.
- Tool results within replayed history: `convertMessages` already emits `functionResponse` parts under `role:"function"` — verify Live accepts these inside `clientContent.turns` during implementation; if rejected, flatten prior tool results into a user-text summary as fallback (implementation detail, note in PR).
- `serverContent.turnComplete` → `EventComplete` with usage from `usageMetadata` (`promptTokenCount`, `candidatesTokenCount`, `cachedContentTokenCount`) and `finishReason` (`STOP`→EndTurn, `MAX_TOKENS`→MaxTokens).
- `interrupted` → treat as ContentStop + Complete (cancel-friendly).
- Rate-limit / quota error strings → existing `shouldRetry` backoff + reconnect (fresh `setup` + history resend).

### Shared helper
`internal/llm/provider/wsutil.go`: dial wrapper (headers/query key), goroutine that closes the conn on `ctx.Done()`, safe channel-close helpers — used by both new files.

## Phase 4 — Model catalog, schema, docs, verification

1. `models.Model`: add `StreamModel string \`json:"stream_model,omitempty"\``.
   - OpenAI entries → `"gpt-realtime"`.
   - Gemini entries → `"gemini-3.1-flash-live-preview"` (single shared default is fine).
2. `agent.go` cost tracking unchanged (`provider.Model()` still reports the chat model — acceptable approximation; note in code comment only if necessary, otherwise no comment).
3. Regenerate schema: `go run cmd/schema/main.go > opencode-schema.json`.
4. README provider/env-var table: trim to OpenAI + Gemini (small doc touch-up, same PR).
5. `.opencode.json` untouched (already has no providers).

## Phase 5 — Rename OpenCode → Dhriti (full scope)

CLI command: **`dhriti`** (lowercase). Display name in prose/help: **Dhriti**.

### 5a. Go module path (touches every import)
- `go.mod`: `module github.com/opencode-ai/opencode` → `module github.com/opencode-ai/dhriti` (keep org path stable; only repo segment changes — avoids inventing an unpushable path).
  - *Alternative if preferred:* `github.com/dhriti/dhriti`. Default to keeping `opencode-ai` org unless user says otherwise at execution time.
- Rewrite all imports: `github.com/opencode-ai/opencode/...` → `github.com/opencode-ai/dhriti/...` across all `.go` files (~130 files; do with a scripted `rg` + `sed`/`go fmt` pass, not hand edits).
- `.goreleaser.yml`: ldflags path `-X github.com/opencode-ai/opencode/internal/version...` → dhriti path; `project_name: opencode` → `dhriti`; archive/binary names `opencode-` → `dhriti-`; brew/aur references.

### 5b. CLI + app identity
- `cmd/root.go`: `Use: "opencode"` → `Use: "dhriti"`; all `Example:` lines `opencode …` → `dhriti …`; `Long` text "OpenCode …" → "Dhriti …".
- `internal/config/config.go`:
  - `appName = "opencode"` → `"dhriti"` (drives viper config-name discovery: `.opencode.json` → `.dhriti.json`, `$XDG_CONFIG_HOME/opencode/` → `.../dhriti/`).
  - `defaultDataDirectory = ".opencode"` → `".dhriti"`.
  - contextPaths: `opencode.md`/`opencode.local.md`/`OpenCode.md`/`OpenCode.local.md`/`OPENCODE.md`/`OPENCODE.local.md` → same names with `dhriti`/`Dhriti`/`DHRITI` casing.
  - `OPENCODE_DEV_DEBUG` → `DHRITI_DEV_DEBUG`.
  - `viper.SetDefault("tui.theme", "opencode")` → `"dhriti"` (check `internal/tui/theme` for the matching theme key registration and rename there too).
- `internal/db/connect.go`: `opencode.db` → `dhriti.db`.
- `internal/fileutil/fileutil.go`: hidden-dir entry `".opencode"` → `".dhriti"`.
- `internal/diff/diff.go`: `<style name="opencode-theme">` → `dhriti-theme` (must match theme rename above).
- `cmd/schema/main.go`: title/description "OpenCode …" → "Dhriti …"; default data dir `.opencode` → `.dhriti`; contextPaths enum; shell enum `"opencode"` → `"dhriti"` (line ~108–110).
- `internal/version/version.go`: comment install path.

### 5c. Repo files
- `opencode-schema.json` → rename to **`dhriti-schema.json`** (regenerated in Phase 4 anyway — regenerate into new filename).
- `.opencode.json` (repo root) → **`.dhriti.json`**; update its `$schema` ref.
- `README.md`: title, install commands, usage examples, config paths, all prose "OpenCode" → "Dhriti", binary names.
- `cmd/schema/README.md`: same treatment.
- Leave `LICENSE`, `.github/`, `install` script contents reviewed for binary/repo name references (`install` script likely hardcodes `opencode` — update artifact names).

### 5d. Prompt / TUI strings
- `internal/llm/prompt/coder.go`: "OpenCode CLI", "You are OpenCode", `OpenCode.md` references → Dhriti equivalents (both prompt variants).
- `internal/tui/tui.go` line ~935: `opencode.md` references in instructions text → `dhriti.md`.
- Grep remaining user-visible "opencode"/"OpenCode" in `internal/tui/**` and update.

### 5e. Out of scope for rename (explicitly keep)
- `github.com/opencode-ai/opencode` references **inside** vendored/quoted external URLs (README upstream links to the original project, license attribution) — update install/clone instructions to local build, but keep "gratefully acknowledges" upstream section wording as attribution to OpenCode.
- Third-party module paths (`openai-go`, `genai`, etc.) obviously untouched.

### Order of operations note
Run Phase 5 **last** (after Phases 1–4 compile), so the mass import rewrite doesn't interleave with provider deletions: single `rg 'opencode-ai/opencode' --files-with-matches | xargs sed` pass, then `gofmt`, then rename schema/config files, then rebuild.

## Verification

- `go build ./...`
- `go vet ./...`
- `go test ./...` (existing `prompt_test`, `ls_test` — no provider-specific tests exist)
- Grep leftover refs: `anthropic|copilot|bedrock|groq|openrouter|vertexai|xai|ProviderLocal|ProviderAzure` across `*.go` → zero (except harmless prompt prose / contextPaths like `.github/copilot-instructions.md`).
- Grep leftover refs: `opencode-ai/opencode` in `*.go`/`go.mod` → zero; user-visible `OpenCode` outside attribution sections → zero.
- `go run cmd/schema/main.go > dhriti-schema.json` succeeds; schema title says Dhriti, providers enum = openai+gemini.
- Manual smoke (if keys available): `OPENAI_API_KEY=… dhriti` → stream a message (Realtime); `GEMINI_API_KEY=…` → same (Live); confirm title generation still works (HTTP `send()`); `dhriti --help` shows Dhriti branding; config discovered at `./.dhriti.json`.

## Risks / open points (non-blocking)

- **Gemini Live history + functionResponse parts**: if `clientContent.turns` rejects `role:"function"`, fallback = flatten tool results into text (will note in PR).
- **Realtime tool schema size**: many tools + big JSON schemas are accepted but watch for `session.update` errors; map to `EventError`.
- **Usage/cost**: streamed tokens billed against realtime/live pricing while tracked under chat-model rates — approximate, matches user's accepted tradeoff.
- **Model freshness**: stream model IDs pinned at plan time (Sep 2026); both are constants in one place (`StreamModel` / provider defaults) so bumping is a one-line change.
