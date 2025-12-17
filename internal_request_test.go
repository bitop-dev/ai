package ai

import (
	"testing"

	"github.com/bitop-dev/ai/openai"
)

func TestToProviderRequestMapping(t *testing.T) {
	model := openai.Chat("gpt-test")

	maxTokens := 123
	temp := float32(0.2)
	topP := float32(0.9)
	stop := []string{"a", "b"}
	metadata := map[string]string{"k": "v"}

	req, err := toProviderRequest(BaseRequest{
		Model: model,
		Messages: []Message{
			System("sys"),
			User("user"),
			{Role: RoleAssistant, Content: []ContentPart{TextPart{Text: "hi"}, ToolCallPart{ID: "1", Name: "t", Args: []byte(`{"x":1}`)}}},
		},
		Tools: []Tool{
			{
				Name:        "tool1",
				Description: "d",
				InputSchema: JSONSchema([]byte(`{"type":"object"}`)),
			},
		},
		MaxTokens:   &maxTokens,
		Temperature: &temp,
		TopP:        &topP,
		Stop:        stop,
		Metadata:    metadata,
	})
	if err != nil {
		t.Fatal(err)
	}

	if req.Model != "gpt-test" {
		t.Fatalf("Model=%q", req.Model)
	}
	if req.ProviderData == nil {
		t.Fatalf("expected ProviderData for openai model")
	}
	if req.MaxTokens == nil || *req.MaxTokens != maxTokens {
		t.Fatalf("MaxTokens mismatch")
	}
	if req.Temperature == nil || *req.Temperature != temp {
		t.Fatalf("Temperature mismatch")
	}
	if req.TopP == nil || *req.TopP != topP {
		t.Fatalf("TopP mismatch")
	}
	if len(req.Stop) != 2 || req.Stop[0] != "a" || req.Stop[1] != "b" {
		t.Fatalf("Stop mismatch: %#v", req.Stop)
	}
	if req.Metadata["k"] != "v" {
		t.Fatalf("Metadata mismatch: %#v", req.Metadata)
	}

	// Ensure clone semantics.
	stop[0] = "changed"
	metadata["k"] = "changed"
	if req.Stop[0] != "a" {
		t.Fatalf("Stop slice was not copied")
	}
	if req.Metadata["k"] != "v" {
		t.Fatalf("Metadata map was not copied")
	}

	if len(req.Messages) != 3 {
		t.Fatalf("Messages len=%d", len(req.Messages))
	}
	if req.Messages[0].Role != "system" {
		t.Fatalf("role[0]=%q", req.Messages[0].Role)
	}
	if req.Messages[1].Role != "user" {
		t.Fatalf("role[1]=%q", req.Messages[1].Role)
	}
	if req.Messages[2].Role != "assistant" {
		t.Fatalf("role[2]=%q", req.Messages[2].Role)
	}

	if len(req.Tools) != 1 || req.Tools[0].Name != "tool1" {
		t.Fatalf("Tools mismatch: %#v", req.Tools)
	}
}
