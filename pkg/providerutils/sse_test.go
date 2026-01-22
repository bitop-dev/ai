package providerutils

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"
)

func TestParseSSEOpenAI(t *testing.T) {
	input := strings.Join([]string{
		"data: {\"id\":\"1\",\"choices\":[{\"delta\":{\"content\":\"hi\"}}]}",
		"",
		"data: {\"id\":\"1\",\"choices\":[{\"delta\":{\"content\":\" there\"}}]}",
		"",
		"data: [DONE]",
		"",
	}, "\n")

	var events []SSEEvent
	options := SSEParseOptions{
		OnEvent: func(event SSEEvent) error {
			events = append(events, event)
			return nil
		},
	}

	if err := ParseSSE(context.Background(), strings.NewReader(input), options); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(events) != 3 {
		t.Fatalf("expected 3 events, got %d", len(events))
	}
	if events[0].Event != "" || events[0].Data == "" {
		t.Fatalf("unexpected event data: %#v", events[0])
	}
	if events[2].Data != "[DONE]" {
		t.Fatalf("expected done marker, got %q", events[2].Data)
	}
}

func TestParseSSEAnthropic(t *testing.T) {
	input := strings.Join([]string{
		"event: message_start",
		"data: {\"type\":\"message_start\"}",
		"",
		"event: content_block_delta",
		"data: {\"type\":\"content_block_delta\",\"delta\":{\"text\":\"hello\"}}",
		"",
		"event: message_stop",
		"data: {\"type\":\"message_stop\"}",
		"",
	}, "\n")

	var events []SSEEvent
	if err := ParseSSE(context.Background(), strings.NewReader(input), SSEParseOptions{
		OnEvent: func(event SSEEvent) error {
			events = append(events, event)
			return nil
		},
	}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(events) != 3 {
		t.Fatalf("expected 3 events, got %d", len(events))
	}
	if events[0].Event != "message_start" {
		t.Fatalf("unexpected event name: %q", events[0].Event)
	}
	if !strings.Contains(events[1].Data, "content_block_delta") {
		t.Fatalf("unexpected event data: %q", events[1].Data)
	}
}

func TestParseSSEChunkHook(t *testing.T) {
	input := strings.Join([]string{
		"event: message",
		"data: first",
		"data: second",
		"",
	}, "\n")

	var chunks []string
	var eventData string
	options := SSEParseOptions{
		OnChunk: func(chunk SSEChunk) error {
			chunks = append(chunks, chunk.Data)
			return nil
		},
		OnEvent: func(event SSEEvent) error {
			eventData = event.Data
			return nil
		},
	}

	if err := ParseSSE(context.Background(), strings.NewReader(input), options); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(chunks) != 2 {
		t.Fatalf("expected 2 chunks, got %d", len(chunks))
	}
	if eventData != "first\nsecond" {
		t.Fatalf("unexpected event data: %q", eventData)
	}
}

func TestWriteSSE(t *testing.T) {
	retry := 2 * time.Second
	event := SSEEvent{Event: "message", Data: "hello\nworld", ID: "evt-1", Retry: &retry}

	var buffer bytes.Buffer
	if err := WriteSSE(&buffer, event); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := strings.Join([]string{
		"event: message",
		"id: evt-1",
		"retry: 2000",
		"data: hello",
		"data: world",
		"",
		"",
	}, "\n")

	if buffer.String() != expected {
		t.Fatalf("unexpected output:\n%q", buffer.String())
	}
}
