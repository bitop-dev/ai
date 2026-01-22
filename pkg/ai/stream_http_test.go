package ai

import (
	"context"
	"errors"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/bitop-dev/ai/pkg/provider"
)

func TestPipeStreamWritesSSE(t *testing.T) {
	streamCtx, streamCancel := context.WithCancel(context.Background())
	ch := make(chan provider.StreamPart, 2)
	ch <- provider.StreamPart{Type: provider.StreamPartTypeTextDelta, TextDelta: &provider.TextDelta{Delta: "Hello"}}
	ch <- provider.StreamPart{Type: provider.StreamPartTypeFinish, Finish: &provider.Finish{Reason: provider.FinishReasonStop}}
	close(ch)

	stream := newStream(streamCtx, streamCancel, ch)
	recorder := httptest.NewRecorder()

	if err := PipeStream(context.Background(), recorder, stream); err != nil {
		t.Fatalf("PipeStream returned error: %v", err)
	}

	if got := recorder.Header().Get("Content-Type"); got != "text/event-stream" {
		t.Fatalf("Content-Type mismatch: got %q", got)
	}

	expected := strings.Join([]string{
		"data: {\"type\":\"text-delta\",\"textDelta\":{\"Delta\":\"Hello\"}}",
		"",
		"data: {\"type\":\"finish\",\"finish\":{\"Reason\":\"stop\",\"Usage\":null}}",
		"",
		"",
	}, "\n")

	if recorder.Body.String() != expected {
		t.Fatalf("unexpected SSE output:\n%s", recorder.Body.String())
	}
}

func TestPipeStreamHonorsContextCancel(t *testing.T) {
	streamCtx, streamCancel := context.WithCancel(context.Background())
	stream := newStream(streamCtx, streamCancel, make(chan provider.StreamPart))
	recorder := httptest.NewRecorder()

	ctx, cancel := context.WithCancel(context.Background())
	result := make(chan error, 1)
	go func() {
		result <- PipeStream(ctx, recorder, stream)
	}()

	cancel()

	select {
	case err := <-result:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("expected context cancellation, got %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("PipeStream did not return after cancellation")
	}

	select {
	case <-streamCtx.Done():
	default:
		t.Fatal("expected stream context to be canceled")
	}
}
