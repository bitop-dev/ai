package provider

import "testing"

func TestStreamPartTypes(t *testing.T) {
	cases := []struct {
		name string
		part StreamPart
		want StreamPartType
	}{
		{
			name: "text-delta",
			part: StreamPart{Type: StreamPartTypeTextDelta, TextDelta: &TextDelta{Delta: "hello"}},
			want: StreamPartTypeTextDelta,
		},
		{
			name: "tool-call",
			part: StreamPart{Type: StreamPartTypeToolCall, ToolCall: &ToolCall{ID: "1", Name: "tool"}},
			want: StreamPartTypeToolCall,
		},
		{
			name: "reasoning-end",
			part: StreamPart{Type: StreamPartTypeReasoningEnd, ReasoningEnd: &ReasoningEnd{Text: "done"}},
			want: StreamPartTypeReasoningEnd,
		},
		{
			name: "finish",
			part: StreamPart{Type: StreamPartTypeFinish, Finish: &Finish{Reason: FinishReasonStop}},
			want: StreamPartTypeFinish,
		},
	}

	for _, entry := range cases {
		if entry.part.Type != entry.want {
			t.Fatalf("%s stream part type mismatch: got %q want %q", entry.name, entry.part.Type, entry.want)
		}
	}
}

func TestStreamPartWarnings(t *testing.T) {
	part := StreamPart{
		Type: StreamPartTypeResponseMetadata,
		ResponseMetadata: &ResponseMetadata{
			RequestID:        "req-1",
			HTTPStatus:       200,
			ProviderMetadata: ProviderMetadata{"provider": {"region": "us"}},
		},
		Warnings: []Warning{{Category: WarningCategoryUnsupportedOption, Message: "unsupported"}},
	}

	if len(part.Warnings) != 1 {
		t.Fatalf("expected warnings to be set")
	}
	if part.Warnings[0].Category != WarningCategoryUnsupportedOption {
		t.Fatalf("unexpected warning category: %q", part.Warnings[0].Category)
	}
}
