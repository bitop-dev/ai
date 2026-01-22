package gateway

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/bitop-dev/ai/pkg/provider"
)

func TestGatewayLanguageModelStreamParts(t *testing.T) {
	var requestHeaders http.Header
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestHeaders = r.Header.Clone()
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = fmt.Fprintf(w, "data: %s\n\n", `{"type":"text-delta","textDelta":"Hello"}`)
		_, _ = fmt.Fprintf(w, "data: %s\n\n", `{"type":"text-delta","textDelta":" world"}`)
		_, _ = fmt.Fprintf(w, "data: %s\n\n", `{"type":"finish","finishReason":"stop","usage":{"prompt_tokens":2,"completion_tokens":3}}`)
	}))
	defer server.Close()

	providerInstance := CreateGateway(GatewaySettings{
		BaseURL:    server.URL,
		HTTPClient: server.Client(),
		APIKey:     "token",
	})

	model, err := providerInstance.LanguageModel(provider.ModelID("test-model"))
	if err != nil {
		t.Fatalf("expected model, got %v", err)
	}

	result, err := model.DoStream(context.Background(), provider.LanguageModelV3CallOptions{
		Prompt: provider.Prompt{
			Messages: []provider.ModelMessage{{
				Role:    provider.RoleUser,
				Content: []provider.ContentPart{provider.TextContent{Text: "Hello"}},
			}},
		},
	})
	if err != nil {
		t.Fatalf("expected stream result, got %v", err)
	}

	var parts []provider.StreamPart
	for part := range result.Stream {
		parts = append(parts, part)
	}

	if len(parts) != 3 {
		t.Fatalf("expected 3 stream parts, got %d", len(parts))
	}
	if parts[0].Type != provider.StreamPartTypeTextDelta || parts[0].TextDelta == nil || parts[0].TextDelta.Delta != "Hello" {
		t.Fatalf("unexpected first part: %#v", parts[0])
	}
	if parts[1].Type != provider.StreamPartTypeTextDelta || parts[1].TextDelta == nil || parts[1].TextDelta.Delta != " world" {
		t.Fatalf("unexpected second part: %#v", parts[1])
	}
	if parts[2].Type != provider.StreamPartTypeFinish || parts[2].Finish == nil || parts[2].Finish.Reason != provider.FinishReasonStop {
		t.Fatalf("unexpected finish part: %#v", parts[2])
	}
	if parts[2].Finish.Usage == nil || parts[2].Finish.Usage.PromptTokens != 2 || parts[2].Finish.Usage.CompletionTokens != 3 {
		t.Fatalf("unexpected usage: %#v", parts[2].Finish.Usage)
	}

	if requestHeaders.Get("ai-language-model-specification-version") != "3" {
		t.Fatalf("expected spec header, got %q", requestHeaders.Get("ai-language-model-specification-version"))
	}
	if requestHeaders.Get("ai-language-model-id") != "test-model" {
		t.Fatalf("expected model header, got %q", requestHeaders.Get("ai-language-model-id"))
	}
	if requestHeaders.Get("ai-language-model-streaming") != "true" {
		t.Fatalf("expected streaming header, got %q", requestHeaders.Get("ai-language-model-streaming"))
	}
}

func TestGatewayLanguageModelStreamErrorMapping(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = fmt.Fprint(w, `{"error":{"message":"Invalid API key","type":"authentication_error"}}`)
	}))
	defer server.Close()

	providerInstance := CreateGateway(GatewaySettings{
		BaseURL:    server.URL,
		HTTPClient: server.Client(),
		APIKey:     "token",
	})

	model, err := providerInstance.LanguageModel(provider.ModelID("test-model"))
	if err != nil {
		t.Fatalf("expected model, got %v", err)
	}

	_, err = model.DoStream(context.Background(), provider.LanguageModelV3CallOptions{
		Prompt: provider.Prompt{
			Messages: []provider.ModelMessage{{
				Role:    provider.RoleUser,
				Content: []provider.ContentPart{provider.TextContent{Text: "Hello"}},
			}},
		},
	})
	if err == nil {
		t.Fatalf("expected error")
	}

	var authErr *provider.AuthenticationError
	if !errors.As(err, &authErr) {
		t.Fatalf("expected authentication error, got %T", err)
	}
	if authErr.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected status 401, got %d", authErr.StatusCode)
	}
}
