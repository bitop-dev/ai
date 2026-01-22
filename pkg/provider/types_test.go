package provider

import "testing"

func TestContentPartTypes(t *testing.T) {
	parts := []struct {
		name string
		part ContentPart
		want ContentPartType
	}{
		{name: "text", part: TextContent{Text: "hello"}, want: ContentPartTypeText},
		{name: "tool-call", part: ToolCallContent{ToolCall: ToolCall{ID: "1", Name: "tool", Arguments: map[string]any{"k": "v"}}}, want: ContentPartTypeToolCall},
		{name: "tool-result", part: ToolResultContent{ToolResult: ToolResult{ID: "1", Name: "tool", Result: "ok", IsError: false}}, want: ContentPartTypeToolResult},
		{name: "source", part: SourceContent{Source: Source{ID: "s1", URL: "https://example.com", Title: "Example"}}, want: ContentPartTypeSource},
		{name: "reasoning", part: ReasoningContent{Text: "thinking"}, want: ContentPartTypeReasoning},
		{name: "image", part: ImageContent{URL: "https://example.com/image.png", MediaType: "image/png"}, want: ContentPartTypeImage},
		{name: "file", part: FileContent{Name: "notes.txt", URL: "https://example.com/notes.txt", MediaType: "text/plain"}, want: ContentPartTypeFile},
	}

	for _, entry := range parts {
		if got := entry.part.ContentType(); got != entry.want {
			t.Fatalf("%s content type mismatch: got %q want %q", entry.name, got, entry.want)
		}
	}
}
