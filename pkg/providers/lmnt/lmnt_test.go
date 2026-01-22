package lmnt

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/bitop-dev/ai/pkg/provider"
)

func TestCreateLMNTDefaults(t *testing.T) {
	client := CreateLMNT(Settings{})
	if client.baseURL != DefaultBaseURL {
		t.Fatalf("expected default base URL %q, got %q", DefaultBaseURL, client.baseURL)
	}
	if client.providerID != provider.ProviderID(DefaultProviderName) {
		t.Fatalf("expected provider ID %q, got %q", DefaultProviderName, client.providerID)
	}
}

func TestLMNTSpeechModelRequest(t *testing.T) {
	t.Setenv("LMNT_API_KEY", "test-key")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/ai/speech/bytes" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if r.Header.Get("x-api-key") != "test-key" {
			t.Fatalf("missing auth header")
		}
		if r.Header.Get("X-Custom") != "custom" {
			t.Fatalf("missing custom header")
		}
		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode payload: %v", err)
		}
		if payload["model"] != "aurora" {
			t.Fatalf("unexpected model: %#v", payload["model"])
		}
		if payload["text"] != "Hello" {
			t.Fatalf("unexpected text: %#v", payload["text"])
		}
		if payload["voice"] != "voice-123" {
			t.Fatalf("unexpected voice: %#v", payload["voice"])
		}
		if payload["response_format"] != "wav" {
			t.Fatalf("unexpected response_format: %#v", payload["response_format"])
		}
		if payload["language"] != "en" {
			t.Fatalf("unexpected language: %#v", payload["language"])
		}
		if payload["conversational"] != true {
			t.Fatalf("unexpected conversational: %#v", payload["conversational"])
		}
		if payload["length"] != 12.5 {
			t.Fatalf("unexpected length: %#v", payload["length"])
		}
		if payload["seed"] != 4.0 {
			t.Fatalf("unexpected seed: %#v", payload["seed"])
		}
		if payload["speed"] != 1.1 {
			t.Fatalf("unexpected speed: %#v", payload["speed"])
		}
		if payload["temperature"] != 0.5 {
			t.Fatalf("unexpected temperature: %#v", payload["temperature"])
		}
		if payload["top_p"] != 0.8 {
			t.Fatalf("unexpected top_p: %#v", payload["top_p"])
		}
		if payload["sample_rate"] != 16000.0 {
			t.Fatalf("unexpected sample_rate: %#v", payload["sample_rate"])
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("audio"))
	}))
	defer server.Close()

	client := CreateLMNT(Settings{BaseURL: server.URL, Headers: map[string]string{"X-Custom": "custom"}})
	model, err := client.SpeechModel("aurora")
	if err != nil {
		t.Fatalf("speech model: %v", err)
	}
	_, err = model.DoGenerate(context.Background(), provider.SpeechModelV3CallOptions{
		Text:         "Hello",
		Voice:        "voice-123",
		OutputFormat: "wav",
		Language:     "en",
		Speed:        1.2,
		RequestOptions: provider.RequestOptions{
			ProviderOptions: provider.ProviderOptions{
				"lmnt": provider.JSONObject{
					"conversational": true,
					"length":         12.5,
					"seed":           4,
					"speed":          1.1,
					"temperature":    0.5,
					"topP":           0.8,
					"sampleRate":     16000,
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
}

func TestLMNTAuthError(t *testing.T) {
	t.Setenv("LMNT_API_KEY", "test-key")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":{"message":"bad key","code":401}}`))
	}))
	defer server.Close()

	client := CreateLMNT(Settings{BaseURL: server.URL})
	model, err := client.SpeechModel("aurora")
	if err != nil {
		t.Fatalf("speech model: %v", err)
	}
	_, err = model.DoGenerate(context.Background(), provider.SpeechModelV3CallOptions{Text: "Hello"})
	if err == nil {
		t.Fatalf("expected error")
	}
	var authErr *provider.AuthenticationError
	if !errors.As(err, &authErr) {
		t.Fatalf("expected authentication error, got %T", err)
	}
	if authErr.Message != "bad key" {
		t.Fatalf("unexpected error message: %s", authErr.Message)
	}
}
