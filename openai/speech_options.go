package openai

// SpeechOptions provides OpenAI-specific options for speech generation.
// Use via ai.GenerateSpeech ProviderOptions: map[string]any{"openai": openai.SpeechOptions{...}}.
type SpeechOptions struct {
	Format string   `json:"format,omitempty"` // e.g. "mp3", "wav"
	Speed  *float32 `json:"speed,omitempty"`
}
