package provider

import "context"

type TranscriptionProvider interface {
	Transcribe(ctx context.Context, req TranscriptionRequest) (TranscriptionResponse, error)
}

type TranscriptionRequest struct {
	Model string

	AudioBytes []byte
	MediaType  string
	Filename   string

	Headers    map[string]string
	MaxRetries *int

	ProviderOptions any
	ProviderData    any
}

type TranscriptSegment struct {
	ID    int
	Start float64
	End   float64
	Text  string
}

type TranscriptionResponse struct {
	Text string

	Segments          []TranscriptSegment
	Language          string
	DurationInSeconds *float64

	Warnings []string

	ProviderMetadata map[string]any
	RawResponse      []byte
}

type SpeechProvider interface {
	GenerateSpeech(ctx context.Context, req SpeechRequest) (SpeechResponse, error)
}

type SpeechRequest struct {
	Model string
	Text  string
	Voice string

	Language string

	Headers    map[string]string
	MaxRetries *int

	ProviderOptions any
	ProviderData    any
}

type SpeechResponse struct {
	AudioBytes []byte
	MediaType  string

	Warnings []string

	ProviderMetadata map[string]any
	RawResponse      []byte
}
