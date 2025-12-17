# ai

Go AI SDK (v0): OpenAI + OpenAI-compatible Chat Completions with a provider-agnostic public API.

Examples:

- `go run ./examples/generate_text` (requires `OPENAI_API_KEY`)
- `go run ./examples/stream_text` (requires `OPENAI_API_KEY`)
- `go run ./examples/generate_object` (requires `OPENAI_API_KEY`)
- `go run ./examples/stream_object` (requires `OPENAI_API_KEY`)
- `go run ./examples/embed` (requires `OPENAI_API_KEY`)
- `go run ./examples/embed_many` (requires `OPENAI_API_KEY`)
- `go run ./examples/generate_image` (requires `OPENAI_API_KEY`)
- `go run ./examples/multimodal_image` (requires `OPENAI_API_KEY`)
- `go run ./examples/transcribe` (requires `OPENAI_API_KEY` + `AUDIO_PATH`)
- `go run ./examples/generate_speech` (requires `OPENAI_API_KEY`)
- `go run ./examples/agent_steps` (requires `OPENAI_API_KEY`)
- `go run ./examples/mcp_tools` (requires `OPENAI_API_KEY` + `MCP_URL`)
- `go run ./examples/mcp_stream_text` (requires `OPENAI_API_KEY` + `MCP_URL`)

Multimodal notes:

- Chat multimodal message inputs currently support images (OpenAI `image_url` content parts).
- Audio message inputs are not supported by OpenAI Chat Completions (use `Transcribe` / `GenerateSpeech`).

MCP notes:

- `mcp.NewClient(...)` performs the MCP lifecycle handshake (`initialize` + `notifications/initialized`) automatically on first use.
- For Streamable HTTP servers that send notifications/requests to the client, use `client.OnNotification(...)` and `client.Listen(ctx)`.
- `client.ToolsCached(...)` caches tool discovery and is invalidated when `client.Listen(ctx)` receives `notifications/tools/list_changed`.
- `mcp.ClientError` wraps client-side failures (transport/parsing/lifecycle). `mcp.RPCError` is returned for JSON-RPC errors. `mcp.CallToolError` wraps failures from `tools/call`.
- `client.ListenAndAutoRefresh(ctx, mcp.AutoRefreshOptions{...})` re-fetches tools/resources/prompts on `*_list_changed` notifications.
- OAuth: use `mcp.OAuthClientCredentialsProvider` with `mcp.HTTPTransport.AuthProvider` to fetch and refresh bearer tokens.

Tool lifecycle + progress:

- Streaming tool input hooks: set `Tool.OnInputStart` / `Tool.OnInputDelta` / `Tool.OnInputAvailable` and call `StreamText`.
- Tool progress: set `BaseRequest.OnToolProgress` and call `meta.Report(...)` inside tools created via `ai.NewTool` / `ai.NewDynamicTool`.

Steps / agentic loop ergonomics:

- `BaseRequest.OnStepFinish` receives per-step summaries (text/tool calls/tool results/usage/finish reason).
- `BaseRequest.PrepareStep` can modify the messages and active tools per step.
- Conversation continuation: append `resp.Response.Messages` (GenerateText) or `stream.Response().Messages` (StreamText) to your history.

Consistency / client controls:

- Per-request headers/retries/timeout: set `BaseRequest.Headers`, `BaseRequest.MaxRetries`, `BaseRequest.Timeout` for text/object APIs; other endpoints expose the same knobs on their request structs.
