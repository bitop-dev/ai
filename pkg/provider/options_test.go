package provider

import (
	"testing"
	"time"
)

func TestJSONTypes(t *testing.T) {
	value := JSONValue(JSONObject{"status": "ok", "count": 1})
	obj, ok := value.(JSONObject)
	if !ok {
		t.Fatalf("expected JSONObject, got %T", value)
	}
	if got := obj["status"]; got != "ok" {
		t.Fatalf("expected status to be ok, got %v", got)
	}
}

func TestOptionsAndFormats(t *testing.T) {
	options := RequestOptions{
		Headers:        map[string]string{"x-test": "true"},
		Timeout:        5 * time.Second,
		IdempotencyKey: "abc123",
		Metadata:       map[string]any{"request": "unit"},
		ProviderOptions: ProviderOptions{
			"anthropic": JSONObject{"cacheControl": JSONObject{"type": "ephemeral"}},
		},
	}

	if options.Timeout == 0 {
		t.Fatalf("expected timeout to be set")
	}

	format := ResponseFormat{Type: ResponseFormatTypeJSON, Schema: JSONObject{"type": "object"}, Name: "summary"}
	if format.Type != ResponseFormatTypeJSON {
		t.Fatalf("expected json response format")
	}

	choice := ToolChoice{Type: ToolChoiceTypeTool, ToolName: "lookup"}
	if choice.ToolName == "" {
		t.Fatalf("expected tool name to be set")
	}
}
