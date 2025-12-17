package provider

import "context"

type EmbeddingProvider interface {
	Embed(ctx context.Context, req EmbeddingRequest) (EmbeddingResponse, error)
}

type EmbeddingRequest struct {
	Model string

	Inputs []string

	Metadata map[string]string

	Headers map[string]string
	// MaxRetries overrides the provider/client default retries when non-nil.
	MaxRetries *int

	// ProviderOptions is provider-specific configuration (e.g. OpenAI dimensions).
	ProviderOptions any

	ProviderData any
}

type EmbeddingResponse struct {
	Vectors [][]float32
	Usage   Usage

	RawResponse []byte
}
