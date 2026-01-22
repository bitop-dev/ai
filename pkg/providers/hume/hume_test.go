package hume

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/vercel/ai-sdk-go/pkg/provider"
)

func TestCreateHumeDefaults(t *testing.T) {
	client := CreateHume(Settings{})
	if client.baseURL != DefaultBaseURL {
		t.Fatalf("expected default base URL %q, got %q", DefaultBaseURL, client.baseURL)
	}
	if client.providerID != provider.ProviderID(DefaultProviderName) {
		t.Fatalf("expected provider ID %q, got %q", DefaultProviderName, client.providerID)
	}
}

func TestHumeSpeechModelRequest(t *testing.T) {
	t.Setenv("HUME_API_KEY", "test-key")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v0/tts/file" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if r.Header.Get("X-Hume-Api-Key") != "test-key" {
			t.Fatalf("missing auth header")
		}
		if r.Header.Get("X-Custom") != "custom" {
			t.Fatalf("missing custom header")
		}
		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode payload: %v", err)
		}
		format, ok := payload["format"].(map[string]any)
		if !ok {
			t.Fatalf("missing format")
		}
		if format["type"] != "wav" {
			t.Fatalf("unexpected format: %#v", format["type"])
		}
		utterances, ok := payload["utterances"].([]any)
		if !ok || len(utterances) != 1 {
			t.Fatalf("unexpected utterances: %#v", payload["utterances"])
		}
		utterance, ok := utterances[0].(map[string]any)
		if !ok {
			t.Fatalf("unexpected utterance type")
		}
		if utterance["text"] != "Hello" {
			t.Fatalf("unexpected text: %#v", utterance["text"])
		}
		if utterance["description"] != "Friendly" {
			t.Fatalf("unexpected description: %#v", utterance["description"])
		}
		if utterance["speed"] != 1.2 {
			t.Fatalf("unexpected speed: %#v", utterance["speed"])
		}
		voice, ok := utterance["voice"].(map[string]any)
		if !ok {
			t.Fatalf("missing voice")
		}
		if voice["id"] != "voice-123" {
			t.Fatalf("unexpected voice id: %#v", voice["id"])
		}
		if voice["provider"] != "HUME_AI" {
			t.Fatalf("unexpected voice provider: %#v", voice["provider"])
		}
		context, ok := payload["context"].(map[string]any)
		if !ok {
			t.Fatalf("missing context")
		}
		if context["generation_id"] != "gen-123" {
			t.Fatalf("unexpected generation_id: %#v", context["generation_id"])
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("audio"))
	}))
	defer server.Close()

	client := CreateHume(Settings{BaseURL: server.URL, Headers: map[string]string{"X-Custom": "custom"}})
	model, err := client.SpeechModel("default")
	if err != nil {
		t.Fatalf("speech model: %v", err)
	}
	_, err = model.DoGenerate(context.Background(), provider.SpeechModelV3CallOptions{
		Text:         "Hello",
		Voice:        "voice-123",
		OutputFormat: "wav",
		Instructions: "Friendly",
		Speed:        1.2,
		RequestOptions: provider.RequestOptions{
			ProviderOptions: provider.ProviderOptions{
				"hume": provider.JSONObject{
					"context": provider.JSONObject{
						"generationId": "gen-123",
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
}

func TestHumeSpeechModelContextUtterances(t *testing.T) {
	t.Setenv("HUME_API_KEY", "test-key")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode payload: %v", err)
		}
		context, ok := payload["context"].(map[string]any)
		if !ok {
			t.Fatalf("missing context")
		}
		utterances, ok := context["utterances"].([]any)
		if !ok || len(utterances) != 1 {
			t.Fatalf("unexpected context utterances: %#v", context["utterances"])
		}
		utterance, ok := utterances[0].(map[string]any)
		if !ok {
			t.Fatalf("unexpected utterance type")
		}
		if utterance["text"] != "context" {
			t.Fatalf("unexpected utterance text: %#v", utterance["text"])
		}
		if utterance["trailing_silence"] != 0.8 {
			t.Fatalf("unexpected trailing_silence: %#v", utterance["trailing_silence"])
		}
		voice, ok := utterance["voice"].(map[string]any)
		if !ok {
			t.Fatalf("missing context voice")
		}
		if voice["name"] != "Allison" {
			t.Fatalf("unexpected voice name: %#v", voice["name"])
		}
		if voice["provider"] != "CUSTOM_VOICE" {
			t.Fatalf("unexpected voice provider: %#v", voice["provider"])
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("audio"))
	}))
	defer server.Close()

	client := CreateHume(Settings{BaseURL: server.URL})
	model, err := client.SpeechModel("default")
	if err != nil {
		t.Fatalf("speech model: %v", err)
	}
	_, err = model.DoGenerate(context.Background(), provider.SpeechModelV3CallOptions{
		Text: "Hello",
		RequestOptions: provider.RequestOptions{
			ProviderOptions: provider.ProviderOptions{
				"hume": provider.JSONObject{
					"context": provider.JSONObject{
						"utterances": []provider.JSONObject{
							{
								"text":            "context",
								"trailingSilence": 0.8,
								"voice": provider.JSONObject{
									"name":     "Allison",
									"provider": "CUSTOM_VOICE",
								},
							},
						},
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
}

func TestHumeAuthError(t *testing.T) {
	t.Setenv("HUME_API_KEY", "test-key")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":{"message":"bad key","code":401}}`))
	}))
	defer server.Close()

	client := CreateHume(Settings{BaseURL: server.URL})
	model, err := client.SpeechModel("default")
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
