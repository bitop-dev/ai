package mcp

import (
	"context"
	"encoding/json"
	"reflect"
	"testing"

	"github.com/bitop-dev/ai"
)

type fakeTransport struct {
	tools []ToolInfo
}

func (t *fakeTransport) Call(ctx context.Context, req json.RawMessage) (json.RawMessage, error) {
	_ = ctx
	var r rpcRequest
	if err := json.Unmarshal(req, &r); err != nil {
		return nil, err
	}
	switch r.Method {
	case "tools/list":
		out, _ := json.Marshal(rpcResponse{
			JSONRPC: "2.0",
			ID:      r.ID,
			Result:  mustJSON(toolListResult{Tools: t.tools}),
		})
		return out, nil
	case "tools/call":
		var params callToolParams
		b, _ := json.Marshal(r.Params)
		_ = json.Unmarshal(b, &params)
		// Return a single text part for convenience.
		out, _ := json.Marshal(rpcResponse{
			JSONRPC: "2.0",
			ID:      r.ID,
			Result:  mustJSON(CallToolResult{Content: []ToolContentPart{{Type: "text", Raw: mustJSON(map[string]any{"type": "text", "text": "ok"})}}}),
		})
		return out, nil
	default:
		out, _ := json.Marshal(rpcResponse{
			JSONRPC: "2.0",
			ID:      r.ID,
			Error:   &rpcError{Code: -32601, Message: "method not found"},
		})
		return out, nil
	}
}

func (t *fakeTransport) Close() error { return nil }

func mustJSON(v any) json.RawMessage {
	b, _ := json.Marshal(v)
	return b
}

func TestClientTools_PrefixAndAllowDeny(t *testing.T) {
	ft := &fakeTransport{
		tools: []ToolInfo{
			{Name: "a"},
			{Name: "b"},
			{Name: "c"},
		},
	}
	c, err := NewClient(ClientOptions{Transport: ft})
	if err != nil {
		t.Fatal(err)
	}

	tools, err := c.Tools(context.Background(), &ToolsOptions{
		Prefix:       "mcp.",
		AllowedTools: []string{"a", "c"},
		DeniedTools:  []string{"c"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(tools) != 1 {
		t.Fatalf("tools=%d", len(tools))
	}
	if tools[0].Name != "mcp.a" {
		t.Fatalf("tool name=%q", tools[0].Name)
	}
}

func TestClientTools_SchemasOrderingDeterministic(t *testing.T) {
	ft := &fakeTransport{
		tools: []ToolInfo{
			{Name: "alpha"},
			{Name: "beta"},
		},
	}
	c, err := NewClient(ClientOptions{Transport: ft})
	if err != nil {
		t.Fatal(err)
	}

	opts := &ToolsOptions{
		Schemas: map[string]ai.Schema{
			"beta":  ai.JSONSchema([]byte(`{"type":"object"}`)),
			"alpha": ai.JSONSchema([]byte(`{"type":"object"}`)),
		},
	}
	tools, err := c.Tools(context.Background(), opts)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, tt := range tools {
		got = append(got, tt.Name)
	}
	if !reflect.DeepEqual(got, []string{"alpha", "beta"}) {
		t.Fatalf("order=%v", got)
	}
}
