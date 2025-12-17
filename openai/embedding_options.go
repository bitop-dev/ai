package openai

// EmbeddingOptions provides OpenAI-specific options for the embeddings endpoint.
// Use it via ai.Embed/ai.EmbedMany ProviderOptions: map[string]any{"openai": openai.EmbeddingOptions{...}}.
type EmbeddingOptions struct {
	Dimensions     *int   `json:"dimensions,omitempty"`
	EncodingFormat string `json:"encoding_format,omitempty"` // "float" (default) or "base64"
}
