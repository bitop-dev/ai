package deepgram

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/bitop-dev/ai/pkg/provider"
)

func TestCreateDeepgramDefaults(t *testing.T) {
	client := CreateDeepgram(Settings{})
	if client.baseURL != DefaultBaseURL {
		t.Fatalf("expected default base URL %q, got %q", DefaultBaseURL, client.baseURL)
	}
	if client.providerID != provider.ProviderID(DefaultProviderName) {
		t.Fatalf("expected provider ID %q, got %q", DefaultProviderName, client.providerID)
	}
}

func TestDeepgramSpeechModelRequest(t *testing.T) {
	t.Setenv("DEEPGRAM_API_KEY", "test-key")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/speak" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Token test-key" {
			t.Fatalf("missing auth header")
		}
		if r.Header.Get("X-Custom") != "custom" {
			t.Fatalf("missing custom header")
		}
		query := r.URL.Query()
		if query.Get("model") != "aura-2-helena-en" {
			t.Fatalf("unexpected model: %s", query.Get("model"))
		}
		if query.Get("encoding") != "linear16" {
			t.Fatalf("unexpected encoding: %s", query.Get("encoding"))
		}
		if query.Get("container") != "wav" {
			t.Fatalf("unexpected container: %s", query.Get("container"))
		}
		if query.Get("bit_rate") != "48000" {
			t.Fatalf("unexpected bit_rate: %s", query.Get("bit_rate"))
		}
		if query.Get("tag") != "alpha,beta" {
			t.Fatalf("unexpected tag: %s", query.Get("tag"))
		}
		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode payload: %v", err)
		}
		if payload["text"] != "Hello" {
			t.Fatalf("unexpected text: %#v", payload["text"])
		}
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "audio")
	}))
	defer server.Close()

	client := CreateDeepgram(Settings{
		BaseURL: server.URL,
		Headers: map[string]string{"X-Custom": "custom"},
	})
	model, err := client.SpeechModel("aura-2-helena-en")
	if err != nil {
		t.Fatalf("speech model: %v", err)
	}
	_, err = model.DoGenerate(context.Background(), provider.SpeechModelV3CallOptions{
		Text:         "Hello",
		OutputFormat: "wav",
		RequestOptions: provider.RequestOptions{
			ProviderOptions: provider.ProviderOptions{
				"deepgram": provider.JSONObject{
					"bitRate": 48000,
					"tag":     []string{"alpha", "beta"},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
}

func TestDeepgramTranscriptionModelRequest(t *testing.T) {
	t.Setenv("DEEPGRAM_API_KEY", "test-key")
	audio := []byte("audio-bytes")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/listen" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Token test-key" {
			t.Fatalf("missing auth header")
		}
		if r.Header.Get("Content-Type") != "audio/wav" {
			t.Fatalf("missing content type")
		}
		query := r.URL.Query()
		if query.Get("model") != "nova-3" {
			t.Fatalf("unexpected model: %s", query.Get("model"))
		}
		if query.Get("diarize") != "true" {
			t.Fatalf("unexpected diarize: %s", query.Get("diarize"))
		}
		if query.Get("detect_language") != "true" {
			t.Fatalf("unexpected detect_language: %s", query.Get("detect_language"))
		}
		if query.Get("language") != "fr" {
			t.Fatalf("unexpected language: %s", query.Get("language"))
		}
		if query.Get("utterances") != "true" {
			t.Fatalf("unexpected utterances: %s", query.Get("utterances"))
		}
		if query.Get("utt_split") != "0.7" {
			t.Fatalf("unexpected utt_split: %s", query.Get("utt_split"))
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read body: %v", err)
		}
		if !bytes.Equal(body, audio) {
			t.Fatalf("unexpected audio payload")
		}
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `{"results":{"channels":[{"alternatives":[{"transcript":"hi"}]}]}}`)
	}))
	defer server.Close()

	client := CreateDeepgram(Settings{BaseURL: server.URL})
	model, err := client.TranscriptionModel("nova-3")
	if err != nil {
		t.Fatalf("transcription model: %v", err)
	}
	_, err = model.DoGenerate(context.Background(), provider.TranscriptionModelV3CallOptions{
		Audio:     audio,
		MediaType: "audio/wav",
		RequestOptions: provider.RequestOptions{
			ProviderOptions: provider.ProviderOptions{
				"deepgram": provider.JSONObject{
					"detectLanguage": true,
					"language":       "fr",
					"utterances":     true,
					"uttSplit":       0.7,
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("transcribe: %v", err)
	}
}

func TestDeepgramAuthError(t *testing.T) {
	t.Setenv("DEEPGRAM_API_KEY", "test-key")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		fmt.Fprint(w, `{"error":{"message":"bad token","code":401}}`)
	}))
	defer server.Close()

	client := CreateDeepgram(Settings{BaseURL: server.URL})
	model, err := client.SpeechModel("aura-2-helena-en")
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
	if authErr.Message != "bad token" {
		t.Fatalf("unexpected error message: %s", authErr.Message)
	}
}
