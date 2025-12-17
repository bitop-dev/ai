package openai

// TranscriptionOptions provides OpenAI-specific options for transcription.
// Use via ai.Transcribe ProviderOptions: map[string]any{"openai": openai.TranscriptionOptions{...}}.
type TranscriptionOptions struct {
	Prompt                 string   `json:"prompt,omitempty"`
	Language               string   `json:"language,omitempty"`
	Temperature            *float32 `json:"temperature,omitempty"`
	ResponseFormat         string   `json:"response_format,omitempty"`         // e.g. "verbose_json", "json", "text"
	TimestampGranularities []string `json:"timestamp_granularities,omitempty"` // e.g. ["word","segment"]
}
