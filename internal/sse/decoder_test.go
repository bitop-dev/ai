package sse

import (
	"strings"
	"testing"
)

func TestDecoder(t *testing.T) {
	in := strings.Join([]string{
		": comment",
		"",
		"data: hello",
		"",
		"data: line1",
		"data: line2",
		"",
		"event: ignored",
		"data: [DONE]",
		"",
	}, "\n")

	d := NewDecoder(strings.NewReader(in))

	if !d.Next() {
		t.Fatalf("expected first event")
	}
	if got := string(d.Data()); got != "" {
		t.Fatalf("expected empty event after comment boundary, got %q", got)
	}

	if !d.Next() {
		t.Fatalf("expected second event")
	}
	if got := string(d.Data()); got != "hello" {
		t.Fatalf("got %q", got)
	}

	if !d.Next() {
		t.Fatalf("expected third event")
	}
	if got := string(d.Data()); got != "line1\nline2" {
		t.Fatalf("got %q", got)
	}

	if !d.Next() {
		t.Fatalf("expected done event")
	}
	if got := string(d.Data()); got != "[DONE]" {
		t.Fatalf("got %q", got)
	}

	if d.Next() {
		t.Fatalf("expected EOF")
	}
	if err := d.Err(); err != nil {
		t.Fatalf("Err=%v", err)
	}
}
