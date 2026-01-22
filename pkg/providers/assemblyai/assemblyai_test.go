package assemblyai

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
	"time"

	"github.com/vercel/ai-sdk-go/pkg/provider"
)

func TestCreateAssemblyAIDefaults(t *testing.T) {
	client := CreateAssemblyAI(Settings{})
	if client.baseURL != DefaultBaseURL {
		t.Fatalf("expected default base URL %q, got %q", DefaultBaseURL, client.baseURL)
	}
	if client.providerID != provider.ProviderID(DefaultProviderName) {
		t.Fatalf("expected provider ID %q, got %q", DefaultProviderName, client.providerID)
	}
}

func TestAssemblyAITranscriptionFlow(t *testing.T) {
	t.Setenv("ASSEMBLYAI_API_KEY", "test-key")
	audio := []byte("audio-bytes")
	var pollCalls int
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v2/upload":
			if r.Method != http.MethodPost {
				t.Fatalf("unexpected method: %s", r.Method)
			}
			if r.Header.Get("Authorization") != "test-key" {
				t.Fatalf("missing auth header")
			}
			if r.Header.Get("X-Custom") != "custom" {
				t.Fatalf("missing custom header")
			}
			if r.Header.Get("Content-Type") != "application/octet-stream" {
				t.Fatalf("unexpected content type: %s", r.Header.Get("Content-Type"))
			}
			body, err := io.ReadAll(r.Body)
			if err != nil {
				t.Fatalf("read body: %v", err)
			}
			if !bytes.Equal(body, audio) {
				t.Fatalf("unexpected audio payload")
			}
			w.WriteHeader(http.StatusOK)
			fmt.Fprintf(w, `{"upload_url":"%s/uploads/audio"}`, server.URL)
		case "/v2/transcript":
			if r.Method != http.MethodPost {
				t.Fatalf("unexpected method: %s", r.Method)
			}
			var payload map[string]any
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Fatalf("decode payload: %v", err)
			}
			if payload["audio_url"] != server.URL+"/uploads/audio" {
				t.Fatalf("unexpected audio_url: %#v", payload["audio_url"])
			}
			if payload["speech_model"] != "best" {
				t.Fatalf("unexpected speech_model: %#v", payload["speech_model"])
			}
			if payload["content_safety"] != true {
				t.Fatalf("unexpected content_safety: %#v", payload["content_safety"])
			}
			if payload["speaker_labels"] != true {
				t.Fatalf("unexpected speaker_labels: %#v", payload["speaker_labels"])
			}
			if payload["webhook_url"] != "https://example.com" {
				t.Fatalf("unexpected webhook_url: %#v", payload["webhook_url"])
			}
			if payload["summary_type"] != "bullets" {
				t.Fatalf("unexpected summary_type: %#v", payload["summary_type"])
			}
			words, ok := payload["word_boost"].([]any)
			if !ok || len(words) != 2 || words[0] != "alpha" || words[1] != "beta" {
				t.Fatalf("unexpected word_boost: %#v", payload["word_boost"])
			}
			w.WriteHeader(http.StatusOK)
			fmt.Fprint(w, `{"id":"transcript-1","status":"queued"}`)
		case "/v2/transcript/transcript-1":
			pollCalls++
			w.WriteHeader(http.StatusOK)
			if pollCalls < 2 {
				fmt.Fprint(w, `{"id":"transcript-1","status":"processing"}`)
				return
			}
			fmt.Fprint(w, `{"id":"transcript-1","status":"completed"}`)
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	client := CreateAssemblyAI(Settings{
		BaseURL:      server.URL,
		Headers:      map[string]string{"X-Custom": "custom"},
		PollInterval: time.Millisecond,
	})
	model, err := client.TranscriptionModel("best")
	if err != nil {
		t.Fatalf("transcription model: %v", err)
	}
	_, err = model.DoGenerate(context.Background(), provider.TranscriptionModelV3CallOptions{
		Audio: audio,
		RequestOptions: provider.RequestOptions{
			ProviderOptions: provider.ProviderOptions{
				"assemblyai": provider.JSONObject{
					"contentSafety": true,
					"speakerLabels": true,
					"summaryType":   "bullets",
					"webhookUrl":    "https://example.com",
					"wordBoost":     []string{"alpha", "beta"},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("transcribe: %v", err)
	}
	if pollCalls != 2 {
		t.Fatalf("expected 2 poll calls, got %d", pollCalls)
	}
}

func TestAssemblyAIAuthError(t *testing.T) {
	t.Setenv("ASSEMBLYAI_API_KEY", "test-key")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		fmt.Fprint(w, `{"error":{"message":"bad token","code":401}}`)
	}))
	defer server.Close()

	client := CreateAssemblyAI(Settings{BaseURL: server.URL})
	model, err := client.TranscriptionModel("best")
	if err != nil {
		t.Fatalf("transcription model: %v", err)
	}
	_, err = model.DoGenerate(context.Background(), provider.TranscriptionModelV3CallOptions{Audio: []byte("audio")})
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
