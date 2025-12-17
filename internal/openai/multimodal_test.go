package openai

import (
	"encoding/json"
	"testing"

	"github.com/bitop-dev/ai/internal/provider"
)

func TestBuildRequest_MultimodalContentArray(t *testing.T) {
	req := provider.Request{
		Model: "gpt-4o-mini",
		Messages: []provider.Message{
			{
				Role: provider.RoleUser,
				Content: []provider.ContentPart{
					provider.TextPart{Text: "look "},
					provider.ImagePart{URL: "https://example.com/cat.png"},
					provider.TextPart{Text: " please"},
				},
			},
		},
	}

	payload, err := buildRequest(req, false)
	if err != nil {
		t.Fatal(err)
	}
	b, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(b, &decoded); err != nil {
		t.Fatal(err)
	}
	msgs, _ := decoded["messages"].([]any)
	if len(msgs) != 1 {
		t.Fatalf("messages=%d", len(msgs))
	}
	m0, _ := msgs[0].(map[string]any)
	content, ok := m0["content"].([]any)
	if !ok || len(content) != 3 {
		t.Fatalf("expected content array, got %#v", m0["content"])
	}
	if content[1].(map[string]any)["type"] != "image_url" {
		t.Fatalf("expected image_url part, got %#v", content[1])
	}
}

func TestBuildRequest_TextOnlyContentString(t *testing.T) {
	req := provider.Request{
		Model: "gpt-4o-mini",
		Messages: []provider.Message{
			{
				Role: provider.RoleUser,
				Content: []provider.ContentPart{
					provider.TextPart{Text: "hi"},
				},
			},
		},
	}

	payload, err := buildRequest(req, false)
	if err != nil {
		t.Fatal(err)
	}
	b, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(b, &decoded); err != nil {
		t.Fatal(err)
	}
	msgs, _ := decoded["messages"].([]any)
	m0, _ := msgs[0].(map[string]any)
	if _, ok := m0["content"].(string); !ok {
		t.Fatalf("expected content string, got %#v", m0["content"])
	}
}

func TestBuildRequest_AudioPartUnsupported(t *testing.T) {
	req := provider.Request{
		Model: "gpt-4o-mini",
		Messages: []provider.Message{
			{
				Role: provider.RoleUser,
				Content: []provider.ContentPart{
					provider.TextPart{Text: "listen"},
					provider.AudioPart{Format: "wav", Base64: "AA=="},
				},
			},
		},
	}

	_, err := buildRequest(req, false)
	if err == nil {
		t.Fatal("expected error")
	}
}
