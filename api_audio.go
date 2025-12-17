package ai

import (
	"context"
	"fmt"

	internalAudio "github.com/bitop-dev/ai/internal/audio"
	"github.com/bitop-dev/ai/internal/provider"
)

type TranscriptSegment struct {
	ID    int
	Start float64
	End   float64
	Text  string
}

type Transcript struct {
	Text string

	Segments          []TranscriptSegment
	Language          string
	DurationInSeconds *float64

	Warnings []string

	ProviderMetadata map[string]any
	RawResponse      []byte
}

type TranscribeRequest struct {
	Model ModelRef

	AudioBytes  []byte
	AudioBase64 string
	AudioURL    string

	Filename  string
	MediaType string

	Headers    map[string]string
	MaxRetries *int

	ProviderOptions map[string]any
}

type NoTranscriptGeneratedError struct {
	Provider    string
	Cause       error
	RawResponse []byte
}

func (e *NoTranscriptGeneratedError) Error() string {
	if e == nil {
		return ""
	}
	if e.Cause != nil {
		return fmt.Sprintf("%s: no transcript generated: %v", e.Provider, e.Cause)
	}
	return fmt.Sprintf("%s: no transcript generated", e.Provider)
}

func (e *NoTranscriptGeneratedError) Unwrap() error { return e.Cause }

func IsNoTranscriptGenerated(err error) bool {
	_, ok := err.(*NoTranscriptGeneratedError)
	return ok
}

func Transcribe(ctx context.Context, req TranscribeRequest) (*Transcript, error) {
	p, err := providerForModel(req.Model)
	if err != nil {
		return nil, err
	}
	tp, ok := p.(provider.TranscriptionProvider)
	if !ok {
		return nil, fmt.Errorf("provider %q does not support transcription", req.Model.Provider())
	}

	audio, mediaType, filename, err := resolveAudio(ctx, req)
	if err != nil {
		return nil, err
	}

	preq := provider.TranscriptionRequest{
		Model:           req.Model.Name(),
		AudioBytes:      audio,
		MediaType:       mediaType,
		Filename:        filename,
		Headers:         cloneStringMap(req.Headers),
		MaxRetries:      req.MaxRetries,
		ProviderOptions: req.ProviderOptions,
		ProviderData:    nil,
	}
	if c, ok := openAIClientFromModel(req.Model); ok {
		preq.ProviderData = c
	}

	out, err := tp.Transcribe(ctx, preq)
	if err != nil {
		return nil, mapProviderError(err)
	}
	if out.Text == "" {
		return nil, &NoTranscriptGeneratedError{Provider: req.Model.Provider(), RawResponse: out.RawResponse}
	}

	t := &Transcript{
		Text:              out.Text,
		Language:          out.Language,
		DurationInSeconds: out.DurationInSeconds,
		Warnings:          out.Warnings,
		ProviderMetadata:  out.ProviderMetadata,
		RawResponse:       out.RawResponse,
	}
	if len(out.Segments) > 0 {
		t.Segments = make([]TranscriptSegment, len(out.Segments))
		for i, s := range out.Segments {
			t.Segments[i] = TranscriptSegment{
				ID:    s.ID,
				Start: s.Start,
				End:   s.End,
				Text:  s.Text,
			}
		}
	}
	return t, nil
}

func resolveAudio(ctx context.Context, req TranscribeRequest) ([]byte, string, string, error) {
	return internalAudio.ResolveInput(ctx, req.AudioBytes, req.AudioBase64, req.AudioURL, req.MediaType, req.Filename)
}

type SpeechAudio struct {
	AudioData []byte
	MediaType string

	Warnings []string

	ProviderMetadata map[string]any
	RawResponse      []byte
}

type GenerateSpeechRequest struct {
	Model ModelRef

	Text  string
	Voice string

	Language string

	Headers    map[string]string
	MaxRetries *int

	ProviderOptions map[string]any
}

type NoSpeechGeneratedError struct {
	Provider    string
	Cause       error
	RawResponse []byte
}

func (e *NoSpeechGeneratedError) Error() string {
	if e == nil {
		return ""
	}
	if e.Cause != nil {
		return fmt.Sprintf("%s: no speech generated: %v", e.Provider, e.Cause)
	}
	return fmt.Sprintf("%s: no speech generated", e.Provider)
}

func (e *NoSpeechGeneratedError) Unwrap() error { return e.Cause }

func IsNoSpeechGenerated(err error) bool {
	_, ok := err.(*NoSpeechGeneratedError)
	return ok
}

func GenerateSpeech(ctx context.Context, req GenerateSpeechRequest) (*SpeechAudio, error) {
	if req.Text == "" {
		return nil, fmt.Errorf("text is required")
	}
	if req.Voice == "" {
		return nil, fmt.Errorf("voice is required")
	}

	p, err := providerForModel(req.Model)
	if err != nil {
		return nil, err
	}
	sp, ok := p.(provider.SpeechProvider)
	if !ok {
		return nil, fmt.Errorf("provider %q does not support speech generation", req.Model.Provider())
	}

	preq := provider.SpeechRequest{
		Model:           req.Model.Name(),
		Text:            req.Text,
		Voice:           req.Voice,
		Language:        req.Language,
		Headers:         cloneStringMap(req.Headers),
		MaxRetries:      req.MaxRetries,
		ProviderOptions: req.ProviderOptions,
		ProviderData:    nil,
	}
	if c, ok := openAIClientFromModel(req.Model); ok {
		preq.ProviderData = c
	}

	out, err := sp.GenerateSpeech(ctx, preq)
	if err != nil {
		return nil, mapProviderError(err)
	}
	if len(out.AudioBytes) == 0 {
		return nil, &NoSpeechGeneratedError{Provider: req.Model.Provider(), RawResponse: out.RawResponse}
	}

	mt := out.MediaType
	if mt == "" {
		mt = "audio/mpeg"
	}
	return &SpeechAudio{
		AudioData:        out.AudioBytes,
		MediaType:        mt,
		Warnings:         out.Warnings,
		ProviderMetadata: out.ProviderMetadata,
		RawResponse:      out.RawResponse,
	}, nil
}
