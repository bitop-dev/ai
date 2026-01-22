package amazonbedrock

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/bitop-dev/ai/pkg/provider"
)

func TestAmazonBedrockSigningAndRequest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/model/test-model/invoke" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if r.Header.Get("X-Amz-Date") == "" {
			t.Fatalf("missing x-amz-date header")
		}
		if r.Header.Get("X-Amz-Content-Sha256") == "" {
			t.Fatalf("missing x-amz-content-sha256 header")
		}
		if r.Header.Get("X-Amz-Security-Token") != "session-token" {
			t.Fatalf("missing session token header")
		}
		if r.Header.Get("X-Custom") != "custom" {
			t.Fatalf("missing custom header")
		}
		if r.Header.Get("X-Request") != "request" {
			t.Fatalf("missing request header")
		}
		auth := r.Header.Get("Authorization")
		if !strings.HasPrefix(auth, "AWS4-HMAC-SHA256") {
			t.Fatalf("missing authorization header")
		}
		if !strings.Contains(auth, "Credential=test-key/") {
			t.Fatalf("unexpected authorization header: %s", auth)
		}

		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode payload: %v", err)
		}
		if payload["inputText"] != "Hello" {
			t.Fatalf("unexpected input text: %#v", payload["inputText"])
		}
		config, ok := payload["textGenerationConfig"].(map[string]any)
		if !ok {
			t.Fatalf("missing textGenerationConfig")
		}
		if config["maxTokenCount"].(float64) != 10 {
			t.Fatalf("unexpected max token count: %#v", config["maxTokenCount"])
		}
		if config["temperature"].(float64) != 0.5 {
			t.Fatalf("unexpected temperature: %#v", config["temperature"])
		}

		fmt.Fprint(w, `{"outputText":"ok"}`)
	}))
	defer server.Close()

	client := CreateAmazonBedrock(Settings{
		AccessKeyID:     "test-key",
		SecretAccessKey: "test-secret",
		SessionToken:    "session-token",
		Region:          "us-east-1",
		BaseURL:         server.URL,
		Headers:         map[string]string{"X-Custom": "custom"},
		Now: func() time.Time {
			return time.Date(2025, 1, 2, 3, 4, 5, 0, time.UTC)
		},
	})
	model, err := client.LanguageModel("test-model")
	if err != nil {
		t.Fatalf("language model: %v", err)
	}
	_, err = model.DoGenerate(context.Background(), provider.LanguageModelV3CallOptions{
		Prompt:          prompt("Hello"),
		MaxOutputTokens: 10,
		Temperature:     0.5,
		RequestOptions:  provider.RequestOptions{Headers: map[string]string{"X-Request": "request"}},
	})
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
}

func TestAmazonBedrockCredentialsFromFile(t *testing.T) {
	creds := "[default]\naws_access_key_id = file-key\naws_secret_access_key = file-secret\naws_session_token = file-token\n"
	file := writeTempFile(t, creds)
	t.Setenv("AWS_SHARED_CREDENTIALS_FILE", file)
	t.Setenv("AWS_ACCESS_KEY_ID", "")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "")
	t.Setenv("AWS_SESSION_TOKEN", "")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if !strings.Contains(auth, "Credential=file-key/") {
			t.Fatalf("expected file credentials, got %s", auth)
		}
		if r.Header.Get("X-Amz-Security-Token") != "file-token" {
			t.Fatalf("expected session token from file")
		}
		fmt.Fprint(w, `{"outputText":"ok"}`)
	}))
	defer server.Close()

	client := CreateAmazonBedrock(Settings{
		Region:  "us-west-2",
		BaseURL: server.URL,
		Now: func() time.Time {
			return time.Date(2025, 2, 3, 4, 5, 6, 0, time.UTC)
		},
	})
	model, err := client.LanguageModel("file-model")
	if err != nil {
		t.Fatalf("language model: %v", err)
	}
	_, err = model.DoGenerate(context.Background(), provider.LanguageModelV3CallOptions{Prompt: prompt("Hello")})
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
}

func TestAmazonBedrockStreamMapping(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/model/stream-model/invoke-with-response-stream" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		flusher := w.(http.Flusher)

		writeEvent := func(payload bedrockStreamPayload) {
			chunkBytes, err := json.Marshal(payload)
			if err != nil {
				t.Fatalf("marshal payload: %v", err)
			}
			event := map[string]any{
				"chunk": map[string]any{"bytes": base64.StdEncoding.EncodeToString(chunkBytes)},
			}
			eventBytes, err := json.Marshal(event)
			if err != nil {
				t.Fatalf("marshal event: %v", err)
			}
			fmt.Fprintf(w, "%s\n", string(eventBytes))
			flusher.Flush()
		}

		writeEvent(bedrockStreamPayload{OutputText: "Hello "})
		writeEvent(bedrockStreamPayload{
			OutputText:       "world",
			CompletionReason: "stop",
			Usage:            &bedrockUsage{InputTokens: 2, OutputTokens: 3},
		})
	}))
	defer server.Close()

	client := CreateAmazonBedrock(Settings{
		AccessKeyID:     "stream-key",
		SecretAccessKey: "stream-secret",
		Region:          "us-east-2",
		BaseURL:         server.URL,
		Now: func() time.Time {
			return time.Date(2025, 3, 4, 5, 6, 7, 0, time.UTC)
		},
	})
	model, err := client.LanguageModel("stream-model")
	if err != nil {
		t.Fatalf("language model: %v", err)
	}
	result, err := model.DoStream(context.Background(), provider.LanguageModelV3CallOptions{Prompt: prompt("Hello")})
	if err != nil {
		t.Fatalf("stream: %v", err)
	}
	parts := collectParts(result.Stream)
	if len(parts) != 4 {
		t.Fatalf("expected 4 parts, got %d", len(parts))
	}
	if parts[1].TextStart == nil || parts[1].TextStart.Text != "Hello " {
		t.Fatalf("unexpected text start: %#v", parts[1].TextStart)
	}
	if parts[2].TextDelta == nil || parts[2].TextDelta.Delta != "world" {
		t.Fatalf("unexpected text delta: %#v", parts[2].TextDelta)
	}
	if parts[3].Finish == nil || parts[3].Finish.Reason != provider.FinishReasonStop {
		t.Fatalf("unexpected finish: %#v", parts[3].Finish)
	}
	if parts[3].Finish.Usage == nil || parts[3].Finish.Usage.TotalTokens != 5 {
		t.Fatalf("unexpected usage: %#v", parts[3].Finish.Usage)
	}
}

func prompt(text string) provider.Prompt {
	return provider.Prompt{
		Messages: []provider.ModelMessage{{
			Role:    provider.RoleUser,
			Content: []provider.ContentPart{provider.TextContent{Text: text}},
		}},
	}
}

func collectParts(stream <-chan provider.StreamPart) []provider.StreamPart {
	var parts []provider.StreamPart
	for part := range stream {
		parts = append(parts, part)
	}
	return parts
}

func writeTempFile(t *testing.T, contents string) string {
	t.Helper()
	file, err := os.CreateTemp(t.TempDir(), "aws-creds-*.ini")
	if err != nil {
		t.Fatalf("temp file: %v", err)
	}
	if _, err := file.WriteString(contents); err != nil {
		t.Fatalf("write temp file: %v", err)
	}
	if err := file.Close(); err != nil {
		t.Fatalf("close temp file: %v", err)
	}
	return file.Name()
}
