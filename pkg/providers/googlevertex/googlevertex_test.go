package googlevertex

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/bitop-dev/ai/pkg/provider"
	"github.com/bitop-dev/ai/pkg/providerutils"
)

func TestVertexEndpointFromProjectLocation(t *testing.T) {
	client := CreateGoogleVertex(Settings{
		Project:     "demo-project",
		Location:    "us-central1",
		AccessToken: "token",
	})
	endpoint, err := client.endpoint("/models/gemini:generateContent")
	if err != nil {
		t.Fatalf("endpoint: %v", err)
	}
	expected := "https://us-central1-aiplatform.googleapis.com/v1beta1/projects/demo-project/locations/us-central1/publishers/google/models/gemini:generateContent"
	if endpoint != expected {
		t.Fatalf("unexpected endpoint: %s", endpoint)
	}
}

func TestVertexGenerateRequestWithServiceAccount(t *testing.T) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	pemBytes := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(privateKey)})
	credentials := map[string]string{
		"client_email": "vertex@example.com",
		"private_key":  string(pemBytes),
		"project_id":   "demo-project",
	}
	credentialsJSON, err := json.Marshal(credentials)
	if err != nil {
		t.Fatalf("marshal credentials: %v", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/token", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("unexpected method: %s", r.Method)
		}
		if r.Header.Get("Content-Type") != "application/x-www-form-urlencoded" {
			t.Fatalf("missing content type")
		}
		body, _ := io.ReadAll(r.Body)
		values, err := url.ParseQuery(string(body))
		if err != nil {
			t.Fatalf("parse token body: %v", err)
		}
		if values.Get("grant_type") == "" || values.Get("assertion") == "" {
			t.Fatalf("missing token request fields")
		}
		fmt.Fprint(w, `{"access_token":"token-123","expires_in":3600}`)
	})

	mux.HandleFunc("/models/gemini-2.5-flash:generateContent", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer token-123" {
			t.Fatalf("missing auth header")
		}
		if r.Header.Get("X-Custom") != "custom" {
			t.Fatalf("missing custom header")
		}
		if r.Header.Get("X-Request") != "request" {
			t.Fatalf("missing request header")
		}

		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		contents, ok := payload["contents"].([]any)
		if !ok || len(contents) != 1 {
			t.Fatalf("unexpected contents: %#v", payload["contents"])
		}
		userMessage := contents[0].(map[string]any)
		if userMessage["role"] != "user" {
			t.Fatalf("unexpected user role: %#v", userMessage["role"])
		}
		userParts := userMessage["parts"].([]any)
		if userParts[0].(map[string]any)["text"] != "Hello" {
			t.Fatalf("unexpected user text: %#v", userParts[0])
		}
		systemInstruction := payload["systemInstruction"].(map[string]any)
		systemParts := systemInstruction["parts"].([]any)
		if systemParts[0].(map[string]any)["text"] != "You are helpful" {
			t.Fatalf("unexpected system instruction: %#v", systemInstruction)
		}
		toolConfig := payload["toolConfig"].(map[string]any)
		functionConfig := toolConfig["functionCallingConfig"].(map[string]any)
		if functionConfig["mode"] != "ANY" {
			t.Fatalf("unexpected tool config: %#v", toolConfig)
		}
		tools := payload["tools"].([]any)
		tool := tools[0].(map[string]any)
		declarations := tool["functionDeclarations"].([]any)
		declaration := declarations[0].(map[string]any)
		if declaration["name"] != "weather" {
			t.Fatalf("unexpected tool declaration: %#v", declaration)
		}
		generationConfig := payload["generationConfig"].(map[string]any)
		if generationConfig["maxOutputTokens"] != float64(128) {
			t.Fatalf("unexpected max tokens: %#v", generationConfig["maxOutputTokens"])
		}
		if generationConfig["responseMimeType"] != "application/json" {
			t.Fatalf("unexpected response mime type: %#v", generationConfig["responseMimeType"])
		}
		if payload["cachedContent"] != "cachedContents/1" {
			t.Fatalf("unexpected cached content: %#v", payload["cachedContent"])
		}
		if payload["candidateCount"] != float64(2) {
			t.Fatalf("unexpected candidate count: %#v", payload["candidateCount"])
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{}`)
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	client := CreateGoogleVertex(Settings{
		BaseURL:         server.URL,
		CredentialsJSON: string(credentialsJSON),
		TokenURL:        server.URL + "/token",
		Headers:         map[string]string{"X-Custom": "custom"},
	})
	model, err := client.LanguageModel("gemini-2.5-flash")
	if err != nil {
		t.Fatalf("language model: %v", err)
	}
	prompt := provider.Prompt{
		Messages: []provider.ModelMessage{
			{Role: provider.RoleSystem, Content: []provider.ContentPart{provider.TextContent{Text: "You are helpful"}}},
			{Role: provider.RoleUser, Content: []provider.ContentPart{provider.TextContent{Text: "Hello"}}},
		},
	}
	_, err = model.DoGenerate(context.Background(), provider.LanguageModelV3CallOptions{
		Prompt:          prompt,
		MaxOutputTokens: 128,
		ToolChoice:      &provider.ToolChoice{Type: provider.ToolChoiceTypeRequired},
		ResponseFormat: &provider.ResponseFormat{
			Type:   provider.ResponseFormatTypeJSON,
			Schema: provider.JSONObject{"type": "object"},
		},
		ProviderOptions: provider.ProviderOptions{
			"google-vertex": provider.JSONObject{
				"tools":         []providerutils.ToolSpecification{{Name: "weather", Description: "Weather", Parameters: provider.JSONObject{"type": "object"}}},
				"cachedContent": "cachedContents/1",
				"request": provider.JSONObject{
					"candidateCount": 2,
				},
			},
		},
		RequestOptions: provider.RequestOptions{Headers: map[string]string{"X-Request": "request"}},
	})
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
}

func TestVertexStream(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/models/gemini-2.5-flash:streamGenerateContent" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer token" {
			t.Fatalf("missing auth header")
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("x-request-id", "req-123")
		flusher, _ := w.(http.Flusher)

		fmt.Fprint(w, "data: {\"candidates\":[{\"content\":{\"parts\":[{\"text\":\"Hello\"}]}}]}\n\n")
		fmt.Fprint(w, "data: {\"candidates\":[{\"content\":{\"parts\":[{\"text\":\" world\"}]}}]}\n\n")
		fmt.Fprint(w, "data: {\"candidates\":[{\"content\":{\"parts\":[{\"functionCall\":{\"name\":\"weather\",\"args\":{\"city\":\"LA\"}}}]}}]}\n\n")
		fmt.Fprint(w, "data: {\"candidates\":[{\"finishReason\":\"STOP\"}],\"usageMetadata\":{\"promptTokenCount\":2,\"candidatesTokenCount\":3,\"totalTokenCount\":5}}\n\n")
		if flusher != nil {
			flusher.Flush()
		}
	}))
	defer server.Close()

	client := CreateGoogleVertex(Settings{BaseURL: server.URL, AccessToken: "token"})
	model, err := client.LanguageModel("gemini-2.5-flash")
	if err != nil {
		t.Fatalf("language model: %v", err)
	}
	result, err := model.DoStream(context.Background(), provider.LanguageModelV3CallOptions{
		Prompt: provider.Prompt{Messages: []provider.ModelMessage{{Role: provider.RoleUser, Content: []provider.ContentPart{provider.TextContent{Text: "Hi"}}}}},
	})
	if err != nil {
		t.Fatalf("stream: %v", err)
	}
	parts := collectParts(result.Stream)
	if len(parts) != 8 {
		t.Fatalf("expected 8 parts, got %d", len(parts))
	}
	if parts[0].Type != provider.StreamPartTypeStreamStart {
		t.Fatalf("expected stream start, got %v", parts[0].Type)
	}
	if parts[1].TextStart == nil || parts[1].TextStart.Text != "Hello" {
		t.Fatalf("unexpected text start: %#v", parts[1].TextStart)
	}
	if parts[2].TextDelta == nil || parts[2].TextDelta.Delta != " world" {
		t.Fatalf("unexpected text delta: %#v", parts[2].TextDelta)
	}
	if parts[3].ToolInputStart == nil || parts[3].ToolInputStart.Name != "weather" {
		t.Fatalf("unexpected tool input start: %#v", parts[3].ToolInputStart)
	}
	if parts[4].ToolInputDelta == nil || !strings.Contains(parts[4].ToolInputDelta.Delta, "city") {
		t.Fatalf("unexpected tool input delta: %#v", parts[4].ToolInputDelta)
	}
	if parts[6].ToolCall == nil || parts[6].ToolCall.Name != "weather" {
		t.Fatalf("unexpected tool call: %#v", parts[6].ToolCall)
	}
	if parts[6].ToolCall.Arguments["city"] != "LA" {
		t.Fatalf("unexpected tool args: %#v", parts[6].ToolCall.Arguments)
	}
	finish := parts[7].Finish
	if finish == nil || finish.Reason != provider.FinishReasonStop {
		t.Fatalf("unexpected finish: %#v", finish)
	}
	if finish.Usage == nil || finish.Usage.TotalTokens != 5 {
		t.Fatalf("unexpected usage: %#v", finish.Usage)
	}
	if parts[7].ResponseMetadata == nil || parts[7].ResponseMetadata.RequestID != "req-123" {
		t.Fatalf("unexpected response metadata: %#v", parts[7].ResponseMetadata)
	}
}

func collectParts(stream <-chan provider.StreamPart) []provider.StreamPart {
	var parts []provider.StreamPart
	for part := range stream {
		parts = append(parts, part)
	}
	return parts
}
