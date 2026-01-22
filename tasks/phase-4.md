# Phase 4 - gateway package

## Goals
- Port the Vercel Gateway provider used as the default model registry in AI SDK v6.
- Preserve error mapping, metadata, and model discovery behavior.
- Provide a drop-in default provider for `providerId:modelId` resolution.

## Scope
### Provider Factory
- `CreateGateway` function with settings:
  - API key (env `AI_GATEWAY_API_KEY` equivalent).
  - Base URL override (default to `https://gateway.ai.vercel.com`).
  - Custom headers and HTTP client override.
  - Provider name override.

### Model Wrappers
- Language model wrapper for gateway responses.
- Embedding model wrapper.
- Image model wrapper.
- Model settings types mirroring gateway metadata (max tokens, pricing, etc.).

### Gateway Errors
- Parse gateway error responses into typed errors:
  - authentication, invalid request, rate limit, model not found, internal error.
- Preserve gateway error metadata and request IDs.

### Metadata and Credits
- Port gateway metadata fetcher (credits, quotas) if applicable.
- Preserve response headers and request metadata.

### Tools and Provider Options
- Map gateway tool definitions to provider tool types.
- Support provider-specific options for gateway features.

## Deliverables
- Go package `gateway` implementing `ProviderV3` and `GatewayModelId` equivalents.
- Gateway model settings types and helpers.
- Tests for error mapping, metadata extraction, and streaming parsing.
- Example usage with `GenerateText` and `StreamText`.

## Implementation Notes
- Gateway uses the same streaming protocol as OpenAI; reuse SSE parser.
- Keep gateway response mapping in its own internal file to isolate API changes.

## Dependencies
- Phase 2 and Phase 3.

## Open Decisions
- Whether to include gateway metadata fetch APIs in v1.
- Default provider name for gateway vs `gateway` prefix.
