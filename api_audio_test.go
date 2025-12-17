package ai

import (
	"context"
	"testing"

	"github.com/bitop-dev/ai/internal/provider"
)

type fakeTranscriptionProvider struct {
	*fakeProvider
	fn func(call int, req provider.TranscriptionRequest) (provider.TranscriptionResponse, error)
	n  int
}

func (p *fakeTranscriptionProvider) Transcribe(ctx context.Context, req provider.TranscriptionRequest) (provider.TranscriptionResponse, error) {
	_ = ctx
	call := p.n
	p.n++
	if p.fn == nil {
		return provider.TranscriptionResponse{}, nil
	}
	return p.fn(call, req)
}

type fakeSpeechProvider struct {
	*fakeProvider
	fn func(call int, req provider.SpeechRequest) (provider.SpeechResponse, error)
	n  int
}

func (p *fakeSpeechProvider) GenerateSpeech(ctx context.Context, req provider.SpeechRequest) (provider.SpeechResponse, error) {
	_ = ctx
	call := p.n
	p.n++
	if p.fn == nil {
		return provider.SpeechResponse{}, nil
	}
	return p.fn(call, req)
}

func TestTranscribe_Success(t *testing.T) {
	tp := &fakeTranscriptionProvider{}
	tp.fn = func(call int, req provider.TranscriptionRequest) (provider.TranscriptionResponse, error) {
		_ = call
		if req.Model != "whisper-1" {
			t.Fatalf("model=%q", req.Model)
		}
		if len(req.AudioBytes) == 0 {
			t.Fatalf("expected audio bytes")
		}
		return provider.TranscriptionResponse{
			Text:     "hello",
			Language: "en",
			Segments: []provider.TranscriptSegment{{ID: 0, Start: 0, End: 1, Text: "hello"}},
		}, nil
	}
	providerName := registerFakeProvider(t, tp)

	out, err := Transcribe(context.Background(), TranscribeRequest{
		Model:      testModel{provider: providerName, name: "whisper-1"},
		AudioBytes: []byte("fake"),
		Filename:   "a.mp3",
	})
	if err != nil {
		t.Fatal(err)
	}
	if out.Text != "hello" || out.Language != "en" || len(out.Segments) != 1 {
		t.Fatalf("out=%#v", out)
	}
}

func TestGenerateSpeech_Success(t *testing.T) {
	sp := &fakeSpeechProvider{}
	sp.fn = func(call int, req provider.SpeechRequest) (provider.SpeechResponse, error) {
		_ = call
		if req.Text != "hi" || req.Voice != "alloy" {
			t.Fatalf("req=%#v", req)
		}
		return provider.SpeechResponse{AudioBytes: []byte{1, 2, 3}, MediaType: "audio/mpeg"}, nil
	}
	providerName := registerFakeProvider(t, sp)

	out, err := GenerateSpeech(context.Background(), GenerateSpeechRequest{
		Model: testModel{provider: providerName, name: "tts-1"},
		Text:  "hi",
		Voice: "alloy",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(out.AudioData) != 3 {
		t.Fatalf("audio len=%d", len(out.AudioData))
	}
}
