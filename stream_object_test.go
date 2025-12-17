package ai

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/bitop-dev/ai/internal/provider"
)

type fakeStream struct {
	deltas []provider.Delta
	final  *provider.Response
	err    error
	i      int
}

func (s *fakeStream) Next() bool {
	if s.err != nil {
		return false
	}
	if s.i >= len(s.deltas) {
		return false
	}
	s.i++
	return true
}

func (s *fakeStream) Delta() provider.Delta {
	if s.i == 0 || s.i > len(s.deltas) {
		return provider.Delta{}
	}
	return s.deltas[s.i-1]
}

func (s *fakeStream) Final() *provider.Response { return s.final }
func (s *fakeStream) Err() error                { return s.err }
func (s *fakeStream) Close() error              { return nil }

func TestStreamObject_PartialAndFinal(t *testing.T) {
	fp := &fakeProvider{}
	fp.stream = func(call int, req provider.Request) (provider.Stream, error) {
		_ = call
		_ = req

		return &fakeStream{
			deltas: []provider.Delta{
				{ToolCalls: []provider.ToolCallDelta{{Index: 0, Name: "__ai_return_json", ArgumentsDelta: `{"x":`}}},
				{ToolCalls: []provider.ToolCallDelta{{Index: 0, ArgumentsDelta: `1`}}},
				{ToolCalls: []provider.ToolCallDelta{{Index: 0, ArgumentsDelta: `}`}}},
			},
			final: &provider.Response{
				Message: provider.Message{
					Role: provider.RoleAssistant,
					Content: []provider.ContentPart{
						provider.ToolCallPart{ID: "c1", Name: "__ai_return_json", Args: []byte(`{"x":1}`)},
					},
				},
				FinishReason: "stop",
			},
		}, nil
	}
	providerName := registerFakeProvider(t, fp)

	type out struct {
		X int `json:"x"`
	}

	schema := JSONSchema([]byte(`{"type":"object","properties":{"x":{"type":"integer"}},"required":["x"],"additionalProperties":false}`))
	stream, err := StreamObject[out](context.Background(), StreamObjectRequest[out]{
		BaseRequest: BaseRequest{
			Model:    testModel{provider: providerName, name: "m"},
			Messages: []Message{User("x")},
		},
		Schema: schema,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer stream.Close()

	var sawAny bool
	var lastRaw []byte
	var lastPartial map[string]any
	for stream.Next() {
		sawAny = true
		lastRaw = append([]byte(nil), stream.Raw()...)
		lastPartial = stream.Partial()
	}
	if err := stream.Err(); err != nil {
		t.Fatal(err)
	}
	if !sawAny {
		t.Fatalf("expected at least one Next() event")
	}
	if !json.Valid(lastRaw) {
		t.Fatalf("expected final raw to be valid JSON, got %q", string(lastRaw))
	}
	if lastPartial == nil {
		t.Fatalf("expected partial to be non-nil once JSON was valid")
	}
	obj := stream.Object()
	if obj == nil || obj.X != 1 {
		t.Fatalf("final object=%#v", obj)
	}
}
