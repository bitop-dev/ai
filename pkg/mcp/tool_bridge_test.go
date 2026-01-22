package mcp

import (
	"context"
	"testing"

	"github.com/bitop-dev/ai/pkg/provider"
	"github.com/bitop-dev/ai/pkg/providerutils"
)

func TestClientToolsBridge(t *testing.T) {
	transport := newTestTransport()
	client, err := NewClient(ClientConfig{Transport: transport})
	if err != nil {
		t.Fatalf("new client: %v", err)
	}

	ctx := context.Background()
	go respondInitialize(t, transport)
	if err := client.Init(ctx); err != nil {
		t.Fatalf("init client: %v", err)
	}

	go func() {
		message := <-transport.sent
		request := message.(JSONRPCRequest)
		if request.Method != "tools/list" {
			t.Errorf("expected tools/list, got %q", request.Method)
			return
		}
		transport.handlers.OnMessage(JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      request.ID,
			Result: ListToolsResult{
				Tools: []Tool{
					{
						Name:        "search",
						Description: "Search docs",
						InputSchema: provider.JSONObject{"type": "object"},
					},
				},
			},
		})

		message = <-transport.sent
		request = message.(JSONRPCRequest)
		if request.Method != "tools/call" {
			t.Errorf("expected tools/call, got %q", request.Method)
			return
		}
		params, ok := request.Params.(CallToolParams)
		if !ok {
			t.Errorf("expected CallToolParams, got %T", request.Params)
			return
		}
		if params.Name != "search" {
			t.Errorf("expected tool name search, got %q", params.Name)
		}
		transport.handlers.OnMessage(JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      request.ID,
			Result: CallToolResult{
				StructuredContent: provider.JSONObject{"ok": true},
			},
		})
	}()

	toolSet, err := client.Tools(ctx, ToolSetOptions{
		ToolName: func(tool Tool) string {
			return "server." + tool.Name
		},
	})
	if err != nil {
		t.Fatalf("tool bridge: %v", err)
	}
	if len(toolSet.Tools) != 1 {
		t.Fatalf("expected 1 tool, got %d", len(toolSet.Tools))
	}

	if got := toolSet.NameMapper.ProviderName("server.search"); got != "search" {
		t.Fatalf("provider name mismatch: got %q", got)
	}
	if got := toolSet.NameMapper.ToolName("search"); got != "server.search" {
		t.Fatalf("tool name mismatch: got %q", got)
	}

	call := providerutils.ToolCall{ID: "call-1", Name: "server.search", Arguments: map[string]any{"query": "go"}}
	result, err := providerutils.ExecuteTool(ctx, toolSet.Tools[0], call)
	if err != nil {
		t.Fatalf("execute tool: %v", err)
	}
	if result.IsError {
		t.Fatalf("unexpected error result")
	}
	payload, ok := result.Result.(map[string]any)
	if !ok {
		t.Fatalf("expected structured content result, got %T", result.Result)
	}
	if payload["ok"] != true {
		t.Fatalf("unexpected structured content: %#v", payload)
	}

	callPart := StreamPartForToolCall(provider.ToolCall(call))
	if callPart.Type != provider.StreamPartTypeToolCall {
		t.Fatalf("expected tool call stream part")
	}
	resultPart := StreamPartForToolResult(provider.ToolCall(call), CallToolResult{StructuredContent: provider.JSONObject{"ok": true}})
	if resultPart.Type != provider.StreamPartTypeToolResult {
		t.Fatalf("expected tool result stream part")
	}
}
