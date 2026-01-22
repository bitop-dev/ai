# Phase 10 - MCP and tool servers

## Goals
- Port the @ai-sdk/mcp package to Go for MCP client and transport support.
- Bridge MCP tools to the Go tool system and streaming parts.
- Provide examples for stdio and HTTP MCP usage.

## Scope
### MCP Client Core
- JSON-RPC client types and request/response envelopes.
- Tool discovery, invocation, and result mapping.
- Error mapping to provider/tool error types.

### Transports
- Stdio transport (stdin/stdout) with framing helpers.
- HTTP transport (POST + SSE or long-poll) compatible with MCP servers.

### Auth and Sessions
- Optional PKCE helper for auth flows (if needed by MCP servers).
- Session metadata support and request headers.

### Tool Bridging
- Convert MCP tool definitions to `providerutils` tool types.
- Map MCP tool calls/results into stream parts for `StreamText`.

## Deliverables
- Go package `mcp` with client, transports, and tool mapping helpers.
- Example programs in `examples/mcp-stdio` and `examples/mcp-http`.
- Tests covering JSON-RPC, transport framing, and tool mapping.

## Dependencies
- Phase 2 (provider types), Phase 3 (tool helpers), Phase 5 (core APIs).

## Open Decisions
- Whether to include a server implementation or only client support in v1.
- Exact HTTP transport shape (SSE vs polling).
