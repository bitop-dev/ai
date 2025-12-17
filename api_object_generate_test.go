package ai

import (
	"context"
	"strings"
	"testing"

	"github.com/bitop-dev/ai/internal/provider"
)

func TestGenerateObject_ToolCallSuccess(t *testing.T) {
	fp := &fakeProvider{}
	fp.generate = func(call int, req provider.Request) (provider.Response, error) {
		_ = call
		return provider.Response{
			Message: provider.Message{
				Role: provider.RoleAssistant,
				Content: []provider.ContentPart{
					provider.ToolCallPart{ID: "c1", Name: "__ai_return_json", Args: []byte(`{"x":1}`)},
				},
			},
			FinishReason: "stop",
		}, nil
	}
	providerName := registerFakeProvider(t, fp)

	type out struct {
		X int `json:"x"`
	}

	resp, err := GenerateObject[out](context.Background(), GenerateObjectRequest[out]{
		BaseRequest: BaseRequest{
			Model:    testModel{provider: providerName, name: "m"},
			Messages: []Message{User("give x")},
		},
		Schema: JSONSchema([]byte(`{"type":"object","properties":{"x":{"type":"integer"}},"required":["x"],"additionalProperties":false}`)),
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Object.X != 1 {
		t.Fatalf("X=%d", resp.Object.X)
	}
	if string(resp.RawJSON) != `{"x":1}` {
		t.Fatalf("RawJSON=%s", string(resp.RawJSON))
	}
}

func TestGenerateObject_RetryOnInvalidJSON(t *testing.T) {
	fp := &fakeProvider{}
	fp.generate = func(call int, req provider.Request) (provider.Response, error) {
		switch call {
		case 0:
			return provider.Response{
				Message: provider.Message{
					Role: provider.RoleAssistant,
					Content: []provider.ContentPart{
						provider.ToolCallPart{ID: "c1", Name: "__ai_return_json", Args: []byte(`{"x":"no"}`)},
					},
				},
			}, nil
		case 1:
			var sawCorrection bool
			for _, m := range req.Messages {
				if m.Role == provider.RoleSystem {
					for _, p := range m.Content {
						if tp, ok := p.(provider.TextPart); ok && strings.Contains(tp.Text, "previous JSON was invalid") {
							sawCorrection = true
						}
					}
				}
			}
			if !sawCorrection {
				t.Fatalf("second request missing correction prompt")
			}
			return provider.Response{
				Message: provider.Message{
					Role: provider.RoleAssistant,
					Content: []provider.ContentPart{
						provider.ToolCallPart{ID: "c1", Name: "__ai_return_json", Args: []byte(`{"x":2}`)},
					},
				},
			}, nil
		default:
			t.Fatalf("unexpected call %d", call)
			return provider.Response{}, nil
		}
	}
	providerName := registerFakeProvider(t, fp)

	type out struct {
		X int `json:"x"`
	}

	retries := 1
	resp, err := GenerateObject[out](context.Background(), GenerateObjectRequest[out]{
		BaseRequest: BaseRequest{
			Model:    testModel{provider: providerName, name: "m"},
			Messages: []Message{User("give x")},
		},
		Schema:     JSONSchema([]byte(`{"type":"object","properties":{"x":{"type":"integer"}},"required":["x"],"additionalProperties":false}`)),
		MaxRetries: &retries,
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Object.X != 2 {
		t.Fatalf("X=%d", resp.Object.X)
	}
	if len(fp.Requests()) != 2 {
		t.Fatalf("provider calls=%d", len(fp.Requests()))
	}
}

func TestGenerateObject_FallbackJSONOnlyOnToolsUnsupported(t *testing.T) {
	fp := &fakeProvider{}
	fp.generate = func(call int, req provider.Request) (provider.Response, error) {
		switch call {
		case 0:
			return provider.Response{}, provider.ErrToolsUnsupported
		case 1:
			if len(req.Tools) != 0 {
				t.Fatalf("expected no tools in JSON-only request, got %d", len(req.Tools))
			}
			return provider.Response{
				Message: provider.Message{
					Role:    provider.RoleAssistant,
					Content: []provider.ContentPart{provider.TextPart{Text: `{"x":5}`}},
				},
			}, nil
		default:
			t.Fatalf("unexpected call %d", call)
			return provider.Response{}, nil
		}
	}
	providerName := registerFakeProvider(t, fp)

	type out struct {
		X int `json:"x"`
	}

	resp, err := GenerateObject[out](context.Background(), GenerateObjectRequest[out]{
		BaseRequest: BaseRequest{
			Model:    testModel{provider: providerName, name: "m"},
			Messages: []Message{User("give x")},
		},
		Schema: JSONSchema([]byte(`{"type":"object","properties":{"x":{"type":"integer"}},"required":["x"],"additionalProperties":false}`)),
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Object.X != 5 {
		t.Fatalf("X=%d", resp.Object.X)
	}
	if len(fp.Requests()) != 2 {
		t.Fatalf("provider calls=%d", len(fp.Requests()))
	}
}
