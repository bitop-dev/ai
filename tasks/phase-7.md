# Phase 7 - providers/anthropic

## Goals
- Implement the Anthropic messages provider with full tool calling support.
- Match streaming behavior, citations, and provider-specific metadata.
- Preserve AI SDK v6 structured output handling and model capabilities.

## Scope
### Provider Factory
- `CreateAnthropic` with settings:
  - API key (env fallback `ANTHROPIC_API_KEY`).
  - Base URL override (default `https://api.anthropic.com/v1`).
  - Custom headers and HTTP client override.
  - Provider name override.
  - Optional `GenerateID` hook.

### Messages Model
- `/messages` request mapping for prompts and tools.
- Tool mapping for `function` tools, provider tools, and dynamic tools.
- Structured output mode: `output_format` vs JSON tool fallback.
- Thinking/extended thinking settings and option validation.
- Context management support and beta headers.

### MCP and Provider Tools
- MCP tool call/result handling with dynamic tool types.
- Server tool mapping (web_search, web_fetch, code_execution, tool_search).* 
- Ensure tool name mapping is reversible for streaming tool deltas.

### Response Parsing
- Content parts: text, thinking, redacted thinking, tool_use, server_tool_use.
- Citation conversion into `Source` content parts.
- Usage conversion and provider metadata capture.
- Finish reason mapping (stop, max_tokens, tool_use, error).

### Streaming
- SSE parsing with `content_block_start`, `delta`, `stop` handling.
- Emit `reasoning-*` and `tool-*` parts correctly during stream.
- Support raw chunk forwarding for debugging.

## Deliverables
- Go package `providers/anthropic` implementing `ProviderV3`.
- Options structs and validation helpers mirroring TS provider options.
- Tests for request serialization, streaming, tool mapping, citations.

## Implementation Notes
- Centralize beta header computation (context management, structured output, tool streaming).
- Mirror TS cache control validation and warnings.
- Use shared tool name mapping helpers from `providerutils`.

## Dependencies
- Phase 2, Phase 3.
- Phase 5 for integration tests and examples.

## Open Decisions
- How much of MCP and agent skills support is required in v1.
- Whether to expose container/skills settings in the initial API.
