# Phase 8 - remaining provider packages

## Goals
- Provide a staged rollout plan for the remaining providers.
- Keep each provider port isolated in `pkg/providers/<name>`.
- Standardize how provider options and streaming are documented.

## Scope
### Language Model Providers
- google, google-vertex, azure, amazon-bedrock, mistral, cohere, groq, togetherai, xai, deepseek.
- cerebras, fireworks, perplexity, openai-compatible, deepinfra, vercel.

### Image Providers
- replicate, fal, prodia, baseten, black-forest-labs, luma.

### Speech and Transcription
- deepgram, assemblyai, elevenlabs, hume, lmnt, gladia, revai.

### Other Packages
- huggingface, langchain, llamaindex.
- MCP adapters and tool servers (handled in Phase 10).

### Not Ported Packages
- devtools, codemod, react, vue, svelte, rsc, angular.
- valibot (replaced by JSON schema adapter in `pkg/providerutils`).
- test-server (replaced by `internal/testserver`).

## Provider Port Template
- `provider.go`: factory + settings struct.
- `language_model.go`: request/response mapping.
- `streaming.go`: SSE parsing and stream part mapping.
- `options.go`: provider-specific options and validation.
- `errors.go`: provider error mapping (if unique).
- `usage.go`: usage conversion helpers.
- `tools.go`: provider tool mapping (if applicable).
- `doc.go`: usage documentation.

### Provider Port Checklist
- Settings struct covers auth, base URL, headers, and defaults.
- Request/response mapping for language models and supported modalities.
- Tool definition mapping and tool call result conversion.
- Streaming mapping for deltas, finish reasons, and usage accumulation.
- Error mapping includes request IDs and provider metadata.
- Usage conversion normalizes tokens, durations, and warnings.
- Options parsing validates provider-specific overrides.
- Tests cover payload construction, response parsing, and streaming.
- Documentation references config, examples, and limitations.

## Deliverables
- Per-provider checklist in `docs/providers/<provider>.md`.
- Implementation order by tier and dependencies.
- Shared template notes for option parsing, streaming, and error handling.
- Explicit notes for `openai-compatible`, `vercel`, and adapter packages.

### Docs Template
- Base outline lives in `docs/providers/template.md`.
- Each provider doc should include configuration, language model usage, tools/structured output, streaming, and known limitations.
- Include a checklist section to track mapping completeness.

## Implementation Tiers
### Tier 1 (Core Language Models)
- google, google-vertex, azure, amazon-bedrock, mistral, cohere, groq, togetherai, xai, deepseek.

### Tier 2 (Compatibility and Secondary LMs)
- openai-compatible, perplexity, cerebras, fireworks, deepinfra, vercel.

### Tier 3 (Image Providers)
- replicate, fal, prodia, baseten, black-forest-labs, luma.

### Tier 4 (Speech/Transcription)
- deepgram, assemblyai, elevenlabs, hume, lmnt, gladia, revai.

### Tier 5 (Adapters)
- huggingface, langchain, llamaindex.

## Dependencies
- Phase 2 and Phase 3 for all providers.
- Phase 5 for integration examples.

## Notes
- Follow the OpenAI/Anthropic provider template for structure and naming.
- Document deviations when a TS package has no Go equivalent.
- Some providers may require extra auth flows (AWS, Azure); document early.

## Open Decisions
- Which providers to prioritize after OpenAI and Anthropic based on demand.
- Whether to split providers into separate Go modules for dependency isolation.
