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
	calls int

	resources []ResourceInfo
	templates []ResourceTemplateInfo
	prompts   []PromptInfo
}

func (t *fakeTransport) Call(ctx context.Context, req json.RawMessage) (json.RawMessage, error) {
	_ = ctx
	t.calls++
	var r rpcRequest
	if err := json.Unmarshal(req, &r); err != nil {
		return nil, err
	}
	switch r.Method {
	case "initialize":
		id := int64(1)
		if r.ID != nil {
			id = *r.ID
		}
		out, _ := json.Marshal(rpcResponse{
			JSONRPC: "2.0",
			ID:      id,
			Result:  mustJSON(InitializeResult{ProtocolVersion: "2025-06-18", ServerInfo: ServerInfo{Name: "s"}}),
		})
		return out, nil
	case "notifications/initialized":
		return mustJSON(rpcResponse{JSONRPC: "2.0", ID: 0, Result: mustJSON(map[string]any{})}), nil
	case "tools/list":
		id := int64(1)
		if r.ID != nil {
			id = *r.ID
		}
		out, _ := json.Marshal(rpcResponse{
			JSONRPC: "2.0",
			ID:      id,
			Result:  mustJSON(toolListResult{Tools: t.tools}),
		})
		return out, nil
	case "tools/call":
		var params callToolParams
		b, _ := json.Marshal(r.Params)
		_ = json.Unmarshal(b, &params)
		// Return a single text part for convenience.
		id := int64(1)
		if r.ID != nil {
			id = *r.ID
		}
		out, _ := json.Marshal(rpcResponse{
			JSONRPC: "2.0",
			ID:      id,
			Result:  mustJSON(CallToolResult{Content: []ToolContentPart{{Type: "text", Raw: mustJSON(map[string]any{"type": "text", "text": "ok"})}}}),
		})
		return out, nil
	case "resources/list":
		id := int64(1)
		if r.ID != nil {
			id = *r.ID
		}
		out, _ := json.Marshal(rpcResponse{
			JSONRPC: "2.0",
			ID:      id,
			Result:  mustJSON(ResourcesListResult{Resources: t.resources}),
		})
		return out, nil
	case "resources/templates/list":
		id := int64(1)
		if r.ID != nil {
			id = *r.ID
		}
		out, _ := json.Marshal(rpcResponse{
			JSONRPC: "2.0",
			ID:      id,
			Result:  mustJSON(ResourceTemplatesListResult{ResourceTemplates: t.templates}),
		})
		return out, nil
	case "prompts/list":
		id := int64(1)
		if r.ID != nil {
			id = *r.ID
		}
		out, _ := json.Marshal(rpcResponse{
			JSONRPC: "2.0",
			ID:      id,
			Result:  mustJSON(PromptsListResult{Prompts: t.prompts}),
		})
		return out, nil
	default:
		id := int64(0)
		if r.ID != nil {
			id = *r.ID
		}
		out, _ := json.Marshal(rpcResponse{
			JSONRPC: "2.0",
			ID:      id,
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

func TestClientToolsCached_CachesAndInvalidatesOnListChanged(t *testing.T) {
	ft := &fakeTransport{
		tools: []ToolInfo{{Name: "a"}},
	}
	c, err := NewClient(ClientOptions{Transport: ft})
	if err != nil {
		t.Fatal(err)
	}

	_, err = c.ToolsCached(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	firstCalls := ft.calls

	// Second call should be cached (no new transport call).
	_, err = c.ToolsCached(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if ft.calls != firstCalls {
		t.Fatalf("expected cached tools, calls=%d first=%d", ft.calls, firstCalls)
	}

	// Simulate server list change notification.
	c.invalidateCaches("notifications/tools/list_changed")

	_, err = c.ToolsCached(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if ft.calls == firstCalls {
		t.Fatalf("expected cache miss after invalidation")
	}
}

func TestListCaches_InvalidateOnNotifications(t *testing.T) {
	ft := &fakeTransport{
		resources: []ResourceInfo{{URI: "file:///a"}},
		templates: []ResourceTemplateInfo{{URITemplate: "file:///{path}"}},
		prompts:   []PromptInfo{{Name: "p"}},
	}
	c, err := NewClient(ClientOptions{Transport: ft})
	if err != nil {
		t.Fatal(err)
	}

	_, _ = c.ListResourcesCached(context.Background())
	_, _ = c.ListResourceTemplatesCached(context.Background())
	_, _ = c.ListPromptsCached(context.Background())
	callsBefore := ft.calls

	_, _ = c.ListResourcesCached(context.Background())
	_, _ = c.ListResourceTemplatesCached(context.Background())
	_, _ = c.ListPromptsCached(context.Background())
	if ft.calls != callsBefore {
		t.Fatalf("expected cached lists, calls=%d before=%d", ft.calls, callsBefore)
	}

	c.invalidateCaches("notifications/resources/list_changed")
	c.invalidateCaches("notifications/prompts/list_changed")

	_, _ = c.ListResourcesCached(context.Background())
	_, _ = c.ListResourceTemplatesCached(context.Background())
	_, _ = c.ListPromptsCached(context.Background())
	if ft.calls == callsBefore {
		t.Fatalf("expected cache misses after invalidation")
	}
}
