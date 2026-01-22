# Phase 2 - provider package (interfaces and errors)

## Goals
- Define the canonical provider interfaces and shared types for the Go SDK.
- Preserve AI SDK v6 semantics for requests, responses, warnings, and metadata.
- Establish a typed error hierarchy with portable error classification.

## Scope
### Interfaces
- Language models: `LanguageModelV3` with `DoGenerate` and `DoStream`.
- Embedding models: `EmbeddingModelV3` with `DoEmbed`.
- Image models: `ImageModelV3` with `DoGenerate`.
- Speech and transcription: `SpeechModelV3`, `TranscriptionModelV3`.
- Reranking models: `RerankingModelV3`.
- Provider: `ProviderV3` with accessors per model type.

### Core Types
- Prompt types: `Prompt`, `ModelMessage`, `ContentPart` variants.
- Unified content types: `TextContent`, `ToolCallContent`, `ToolResultContent`, `SourceContent`, `ReasoningContent`, `ImageContent`, `FileContent`.
- Call options: standard settings (max output tokens, temperature, topP/topK, stop sequences), tool settings, response format.
- Request options: headers, timeout, idempotency key, and per-call metadata.
- Tool choice types: `auto`, `none`, `required`, and specific tool selection.
- Provider options: `map[string]any` for provider-specific settings.
- Usage types: `LanguageModelUsage`, `EmbeddingUsage`, `ImageUsage`.
- Warnings: `Warning` with standard categories and optional metadata.
- Provider metadata maps and response headers.
- Stable identifiers: `ProviderID`, `ModelID`, and parser for `providerId:modelId` strings.

### Error Model
- Base error type `AISDKError` with message, cause, and optional details.
- API errors: `ApiCallError`, `AuthenticationError`, `RateLimitError`, `InvalidRequestError`, `InternalServerError`.
- SDK errors: `InvalidPromptError`, `InvalidResponseDataError`, `NoSuchModelError`, `UnsupportedFunctionalityError`.
- Error helpers: `Is`/`As` compatibility, typed sentinel values for category matching.

### JSON Types
- `JSONValue` and `JSONObject` for schema-compatible data.
- `JSONSchema` type alias or interface for schema-aware functions.

## Detailed Decisions
### Streaming Types
- Stream parts as a discriminated union struct with `Type` and payload fields.
- Standard events: `StreamStart`, `TextStart`, `TextDelta`, `TextEnd`, `ToolInputStart`, `ToolInputDelta`, `ToolInputEnd`, `ToolCall`, `ToolResult`, `Source`, `ReasoningStart`, `ReasoningDelta`, `ReasoningEnd`, `ResponseMetadata`, `Finish`, `Error`.
- Include optional `ProviderMetadata` and `Warnings` on relevant parts.
- Allow optional raw provider chunk data for debugging.

### Provider Metadata
- Use `map[string]any` or a typed struct with `map[string]map[string]any` for provider-specific metadata.
- Ensure metadata is preserved across core APIs and streaming.

### Response Format
- Support `ResponseFormat` with `type=json` and optional schema.
- Provide schema validation hooks but do not enforce provider-specific JSON Schema capabilities in this package.
- Add strict vs lenient parse modes for structured output helpers.

## Deliverables
- Go package `provider` with interfaces, types, and error definitions.
- `doc.go` and README snippets for provider authors.
- Compatibility notes about which AI SDK v6 features are fully supported.
- `RequestOptions`, `ToolChoice`, and `ProviderOptions` types.

## Dependencies
- Phase 1.

## Open Decisions
- Whether to include v2 compatibility interfaces or require v3-only providers.
- JSON schema validation library choice for schema-aware helpers.
- Exact streaming union representation (struct + Type string vs interface hierarchy).
