# PRD: Go AI SDK (Functional, Chat Completions) — v0

Repo: `https://github.com/bitop-dev/ai`
Scope: **OpenAI + OpenAI-compatible**, **Chat Completions only**, functional DX similar to Vercel AI SDK, Go-idiomatic, extensible.

Note: v0 targets Chat Completions only; message content is designed to extend to images/audio in v0.x/v1, and embeddings can be added later as a separate API without breaking changes.

---

## 1) Public API (stable)

### 1.1 GenerateText

```go
func GenerateText(ctx context.Context, req GenerateTextRequest) (*GenerateTextResponse, error)
```

### 1.2 StreamText

```go
func StreamText(ctx context.Context, req StreamTextRequest) (*TextStream, error)
```

### 1.3 GenerateObject

```go
func GenerateObject[T any](ctx context.Context, req GenerateObjectRequest[T]) (*GenerateObjectResponse[T], error)
```

### 1.4 StreamObject (optional v0.1, not required in v0)

Not in initial acceptance criteria unless you explicitly want it.

If implemented:

```go
func StreamObject[T any](ctx context.Context, req StreamObjectRequest[T]) (*ObjectStream[T], error)
```

```go
type StreamObjectRequest[T any] = GenerateObjectRequest[T]

type ObjectStream[T any] struct {
  Next() bool
  Partial() map[string]any    // best-effort; may be nil until parseable
  Raw() json.RawMessage       // accumulated args so far; may be invalid mid-stream
  Object() *T                 // final decoded object once done
  Err() error
  Close() error
}
```

---

## 2) Requests and responses

### 2.1 Common request fields

```go
type BaseRequest struct {
  Model ModelRef

  Messages []Message
  Tools    []Tool // optional
  ToolLoop *ToolLoopOptions // optional; defaults applied when nil

  MaxTokens   *int
  Temperature *float32
  TopP        *float32
  Stop        []string

  Metadata map[string]string
}
```

### 2.2 GenerateTextRequest / Response

```go
type GenerateTextRequest struct {
  BaseRequest
}

type GenerateTextResponse struct {
  Text string

  Message Message
  Usage   Usage

  FinishReason FinishReason
}
```

Text extraction rules:

* If the final assistant message has multiple content parts, `Text` is the concatenation of all `TextPart`s in order.
* If no text parts, `Text` is `""` (do not invent text).

### 2.3 StreamTextRequest / TextStream

```go
type StreamTextRequest = GenerateTextRequest

type TextStream struct {
  Next() bool
  Delta() string        // only valid after Next()==true
  Message() *Message    // available once stream is done
  Err() error
  Close() error
}
```

Streaming rules:

* `Next()` blocks for the next event/delta; returns false on EOF or error.
* `Delta()` returns only newly arrived text (not cumulative).
* Tool call deltas are handled internally; caller only gets text deltas (v0).

  * Full event stream API can be added later without breaking.

Tool + streaming interaction (v0):

* If the model emits tool calls while streaming, the SDK must transparently run the tool loop and continue streaming subsequent assistant text.
* The caller continues to receive only assistant text deltas across these internal iterations.

---

## 3) GenerateObject: typed structured output

### 3.1 GenerateObjectRequest / Response

```go
type GenerateObjectRequest[T any] struct {
  BaseRequest

  // One of these must be provided:
  Schema Schema            // JSON schema (authoritative)
  // or:
  Example *T               // optional hint used to derive schema (if implemented)

  // Strictness controls:
  Strict *bool             // default true: fail if response not valid
  MaxRetries *int          // default 1: re-ask model to fix invalid JSON
}
```

```go
type GenerateObjectResponse[T any] struct {
  Object T

  RawJSON json.RawMessage  // exact JSON returned/parsed
  Message Message
  Usage   Usage
  FinishReason FinishReason

  ValidationError error    // set when Strict=false and validation fails
}
```

### 3.2 Behavior contract

Goal: reliably get a JSON object of type `T` using Chat Completions.

**Primary mechanism (v0):**

* Use tool-calling as the enforcement mechanism:

  * SDK injects a synthetic tool (internal) named `__ai_return_json`
  * Tool schema is the requested JSON schema
  * SDK asks the model to call that tool with the object as arguments
  * SDK captures the tool call args as the JSON payload

This is the most portable approach across “OpenAI-compatible” servers because tool calling is widely mirrored; it also avoids fragile “please output JSON” prompting.

Concrete v0 decisions:

* When `Schema` is provided, `GenerateObject` must inject the synthetic tool (even if the caller provided other tools).
* Synthetic tool name: `__ai_return_json` (stable).
* If the caller provides a tool with the same name, `GenerateObject` returns an error (name collision).
* `GenerateObject` includes user tools in the loop; completion occurs when the synthetic tool is called and its args validate.

If the provider/server does not support tools:

* Fallback to “JSON-only” prompting:

  * System instruction: “Return ONLY valid JSON matching this schema…”
  * Parse the assistant text as JSON
  * Validate against schema if available

**Validation:**

* If `Schema` is provided: validate output JSON against schema.
* If no schema is provided in v0: validation is “JSON parse + decode into T” only.

Implementation choice (v0):

* Use `github.com/santhosh-tekuri/jsonschema/v5` for JSON Schema validation.
* Default to Draft-07 semantics unless `$schema` specifies otherwise.

**Retries:**

* If invalid and `Strict=true`, retry up to `MaxRetries` by appending:

  * A system/assistant correction instruction including validation errors (sanitized)
  * The previous raw output
* If still invalid: return an error (or set `ValidationError` when `Strict=false`).

### 3.3 Schema representation (v0)

```go
type Schema struct {
  JSON json.RawMessage // must be a valid JSON Schema document or fragment
}
```

Helper constructors:

```go
func JSONSchema(raw json.RawMessage) Schema
```

(Go struct -> schema generation can be v1; do not block v0 on it.)

---

## 4) Core types

### 4.1 ModelRef

```go
type ModelRef interface {
  Provider() string
  Name() string
}
```

OpenAI model ref:

```go
openai.Chat("gpt-4.1")
```

Notes:

* `ai.*` resolves providers by `ModelRef.Provider()`. Unknown/unconfigured providers must return an error.
* v0 supports a global default OpenAI client and optional per-client model refs:

  * `openai.Chat(model)` uses the default global client (configured via `openai.Configure`).
  * `client := openai.NewClient(openai.Config{...}); client.Chat(model)` returns a `ModelRef` bound to that client.
* `ai` may use internal (unexported) interfaces/type assertions on `ModelRef` implementations to access provider-specific wiring (e.g. a client handle) without leaking provider types into the public API.

### 4.2 Messages and content

```go
type Role string
const (
  RoleSystem    Role = "system"
  RoleUser      Role = "user"
  RoleAssistant Role = "assistant"
  RoleTool      Role = "tool"
)

type Message struct {
  Role    Role
  Content []ContentPart
  Name    string // optional (used for tool role)
}

type ContentPart interface{ isContentPart() }

type TextPart struct{ Text string }
type ToolCallPart struct {
  ID   string
  Name string
  Args json.RawMessage
}

// Future v0.x/v1 content parts (not required for v0 acceptance criteria):
type ImagePart struct {
  // Provider-specific representation (e.g. URL or base64) may be added later.
}
type AudioPart struct {
  // Provider-specific representation may be added later.
}
```

Helpers:

```go
func System(text string) Message
func User(text string) Message
func Assistant(text string) Message
func ToolResult(toolName string, value any) Message // value JSON-marshaled
```

### 4.3 Usage and finish reason

```go
type FinishReason string

const (
  FinishStop          FinishReason = "stop"
  FinishLength        FinishReason = "length"
  FinishToolCalls     FinishReason = "tool_calls"
  FinishContentFilter FinishReason = "content_filter"
  FinishError         FinishReason = "error"
  FinishUnknown       FinishReason = "unknown"
)

type Usage struct {
  PromptTokens     int
  CompletionTokens int
  TotalTokens      int

  // Optional provider-specific breakdown; may be nil.
  PromptTokensDetails     map[string]int
  CompletionTokensDetails map[string]int
}
```

---

## 5) Tools (user-defined)

```go
type Tool struct {
  Name        string
  Description string
  InputSchema Schema
  Handler     ToolHandler
}

type ToolHandler func(ctx context.Context, input json.RawMessage) (any, error)
```

Tool loop behavior (used by GenerateText when tools exist, and by GenerateObject internally):

* If model returns tool calls:

  * execute handlers (sequential by default; option for parallel later)
  * append tool result messages
  * continue until no tool calls or max iterations reached

Config:

```go
type ToolLoopOptions struct {
  MaxIterations int // default 5
}
```

---

## 6) Provider scope: OpenAI + OpenAI-compatible (only)

### 6.1 OpenAI configuration (v0)

```go
openai.Configure(openai.Config{
  APIKey     string
  BaseURL    string            // default https://api.openai.com
  APIPrefix  string            // default /v1
  Headers    map[string]string // extra headers for compatible servers
  HTTPClient *http.Client

  // Optional retry policy (v0):
  MaxRetries int           // default 2
  MinBackoff time.Duration // default 250ms
  MaxBackoff time.Duration // default 5s
})
```

Notes:

* v0 supports global config for simplicity via `openai.Configure`, plus an optional per-client config:

  * `client := openai.NewClient(openai.Config{...})`
  * `client.Chat(modelName)` (binds the returned `ModelRef` to that client)
* Internals must be written so you can later add per-request/per-client config without breaking the public `ai.*` functions.
* Retry behavior:

  * retry on network timeouts and HTTP `408`, `409`, `429`, `5xx`
  * use exponential backoff with full jitter
  * respect `Retry-After` when present

### 6.2 Endpoint

* `POST {BaseURL}{APIPrefix}/chat/completions`
* Streaming uses `stream=true` and SSE parsing (`data:` frames, `[DONE]`).

### 6.3 OpenAI-compatible requirements

* Must work if the server matches common Chat Completions semantics.
* Be tolerant of missing optional fields.
* Avoid relying on OpenAI-only extensions.

---

## 7) Errors

```go
type Error struct {
  Provider  string
  Code      string
  Status    int
  Message   string
  Retryable bool
  Cause     error
}
```

Helpers:

```go
func IsRateLimited(err error) bool
func IsAuth(err error) bool
func IsTimeout(err error) bool
func IsCanceled(err error) bool
```

---

## 8) Internal architecture (required)

Public packages:

* `ai` (top-level): exports the functional API + core types
* `openai`: exports `Chat(modelName)`, `Configure(Config)`, and `NewClient(Config)`

Internal packages:

```
/internal/provider    // registry + Provider interface
/internal/openai      // OpenAI implementation
/internal/sse         // SSE decoder
/internal/httpx       // request building, headers, retries (optional v0)
```

Internal provider interface (not exported):

```go
type Provider interface {
  Generate(ctx context.Context, req InternalRequest) (InternalResponse, error)
  Stream(ctx context.Context, req InternalRequest) (InternalStream, error)
}
```

`ai.GenerateText` and `ai.GenerateObject` must only talk to `internal/provider`, never to OpenAI directly.

---

## 9) Testing (must-have)

Unit tests:

* request mapping (messages/tools -> chat completion payload)
* SSE parsing (delta content frames, done, error frames)
* tool loop behavior (including tool call args capture for GenerateObject)

Integration tests (env-gated):

* OpenAI GenerateText
* OpenAI StreamText
* OpenAI GenerateObject (schema + strict)

Deterministic fake provider:

* used to test tool loop and GenerateObject retry logic without network.

---

## 10) Acceptance criteria (v0 “ship”)

1. `GenerateText` works with OpenAI Chat Completions.
2. `StreamText` yields incremental text deltas and completes cleanly.
3. `GenerateObject[T]`:

   * returns valid decoded `T` when the model complies
   * enforces schema via internal tool-calling strategy when tools are supported
   * retries on invalid output (bounded)
4. Works against at least one OpenAI-compatible server via `BaseURL`.
5. Public API contains **no OpenAI request/response structs**.

---

## 11) Handoff prompt for AI coding agent

Implement `bitop-dev/ai` as a Go SDK with a functional API: `GenerateText`, `StreamText`, `GenerateObject[T]`. Use OpenAI Chat Completions only, but support OpenAI-compatible servers via configurable BaseURL/APIPrefix/Headers. Keep public API provider-agnostic. Implement SSE streaming parsing. Implement tools and a tool loop. For `GenerateObject`, enforce JSON output using an internal synthetic tool that returns the object as tool-call args; fallback to JSON-only prompting if tools unsupported. Add unit tests (mapping, SSE, tool loop, object retries) and env-gated integration tests. Provide examples for all three functions.
