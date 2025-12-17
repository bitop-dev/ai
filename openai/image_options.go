package openai

// ImageOptions provides OpenAI-specific options for image generation.
// Use via ai.GenerateImage ProviderOptions: map[string]any{"openai": openai.ImageOptions{...}}.
type ImageOptions struct {
	Quality string `json:"quality,omitempty"` // e.g. "hd"
	Style   string `json:"style,omitempty"`   // e.g. "vivid" or "natural"
}
