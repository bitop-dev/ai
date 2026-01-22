package gladia

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/bitop-dev/ai/pkg/provider"
)

func TestCreateGladiaDefaults(t *testing.T) {
	client := CreateGladia(Settings{})
	if client.baseURL != DefaultBaseURL {
		t.Fatalf("expected default base URL %q, got %q", DefaultBaseURL, client.baseURL)
	}
	if client.providerID != provider.ProviderID(DefaultProviderName) {
		t.Fatalf("expected provider ID %q, got %q", DefaultProviderName, client.providerID)
	}
}

func TestGladiaTranscriptionFlow(t *testing.T) {
	t.Setenv("GLADIA_API_KEY", "test-key")
	audio := []byte("audio-bytes")
	var pollCalls int
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v2/upload":
			if r.Method != http.MethodPost {
				t.Fatalf("unexpected method: %s", r.Method)
			}
			if r.Header.Get("x-gladia-key") != "test-key" {
				t.Fatalf("missing auth header")
			}
			if r.Header.Get("X-Custom") != "custom" {
				t.Fatalf("missing custom header")
			}
			fileHeader, fileBytes := readMultipartFile(t, r, "audio")
			if fileHeader.Filename != "audio.mp3" {
				t.Fatalf("unexpected file name: %s", fileHeader.Filename)
			}
			if fileHeader.Header.Get("Content-Type") != "audio/mpeg" {
				t.Fatalf("unexpected content type: %s", fileHeader.Header.Get("Content-Type"))
			}
			if string(fileBytes) != string(audio) {
				t.Fatalf("unexpected audio payload")
			}
			w.WriteHeader(http.StatusOK)
			fmt.Fprintf(w, `{"audio_url":"%s/uploads/audio"}`, server.URL)
		case "/v2/pre-recorded":
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
			if payload["context_prompt"] != "context" {
				t.Fatalf("unexpected context_prompt: %#v", payload["context_prompt"])
			}
			if payload["custom_vocabulary"] != true {
				t.Fatalf("unexpected custom_vocabulary: %#v", payload["custom_vocabulary"])
			}
			if payload["detect_language"] != true {
				t.Fatalf("unexpected detect_language: %#v", payload["detect_language"])
			}
			config, ok := payload["custom_vocabulary_config"].(map[string]any)
			if !ok {
				t.Fatalf("expected custom_vocabulary_config")
			}
			if config["default_intensity"] != 0.5 {
				t.Fatalf("unexpected default_intensity: %#v", config["default_intensity"])
			}
			vocab, ok := config["vocabulary"].([]any)
			if !ok || len(vocab) != 2 {
				t.Fatalf("unexpected vocabulary: %#v", config["vocabulary"])
			}
			if vocab[0] != "alpha" {
				t.Fatalf("unexpected vocab item: %#v", vocab[0])
			}
			vocabEntry, ok := vocab[1].(map[string]any)
			if !ok || vocabEntry["value"] != "beta" {
				t.Fatalf("unexpected vocab entry: %#v", vocab[1])
			}
			codeSwitching, ok := payload["code_switching_config"].(map[string]any)
			if !ok || len(codeSwitching) == 0 {
				t.Fatalf("missing code_switching_config")
			}
			callbackConfig, ok := payload["callback_config"].(map[string]any)
			if !ok || callbackConfig["url"] != "https://example.com/hook" {
				t.Fatalf("unexpected callback_config: %#v", callbackConfig)
			}
			translationConfig, ok := payload["translation_config"].(map[string]any)
			if !ok || translationConfig["model"] != "enhanced" {
				t.Fatalf("unexpected translation_config: %#v", translationConfig)
			}
			w.WriteHeader(http.StatusOK)
			fmt.Fprintf(w, `{"result_url":"%s/results/1"}`, server.URL)
		case "/results/1":
			pollCalls++
			w.WriteHeader(http.StatusOK)
			if pollCalls < 2 {
				fmt.Fprint(w, `{"status":"processing"}`)
				return
			}
			fmt.Fprint(w, `{"status":"done","result":{"metadata":{"audio_duration":2.1},"transcription":{"full_transcript":"hello","languages":["en"],"utterances":[{"start":0,"end":1,"text":"hello"}]}}}`)
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	client := CreateGladia(Settings{
		BaseURL:      server.URL,
		Headers:      map[string]string{"X-Custom": "custom"},
		PollInterval: time.Millisecond,
		PollTimeout:  50 * time.Millisecond,
	})
	model, err := client.TranscriptionModel("default")
	if err != nil {
		t.Fatalf("transcription model: %v", err)
	}
	_, err = model.DoGenerate(context.Background(), provider.TranscriptionModelV3CallOptions{
		Audio:     audio,
		MediaType: "audio/mpeg",
		RequestOptions: provider.RequestOptions{
			ProviderOptions: provider.ProviderOptions{
				"gladia": provider.JSONObject{
					"contextPrompt":    "context",
					"customVocabulary": true,
					"detectLanguage":   true,
					"customVocabularyConfig": provider.JSONObject{
						"vocabulary": []any{
							"alpha",
							provider.JSONObject{"value": "beta"},
						},
						"defaultIntensity": 0.5,
					},
					"codeSwitchingConfig": provider.JSONObject{
						"languages": []string{"en", "es"},
					},
					"callbackConfig": provider.JSONObject{
						"url":    "https://example.com/hook",
						"method": "POST",
					},
					"translationConfig": provider.JSONObject{
						"targetLanguages":         []string{"es"},
						"model":                   "enhanced",
						"matchOriginalUtterances": true,
					},
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

func TestGladiaAuthError(t *testing.T) {
	t.Setenv("GLADIA_API_KEY", "test-key")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		fmt.Fprint(w, `{"error":{"message":"bad token","code":401}}`)
	}))
	defer server.Close()

	client := CreateGladia(Settings{BaseURL: server.URL})
	model, err := client.TranscriptionModel("default")
	if err != nil {
		t.Fatalf("transcription model: %v", err)
	}
	_, err = model.DoGenerate(context.Background(), provider.TranscriptionModelV3CallOptions{
		Audio:     []byte("audio"),
		MediaType: "audio/mpeg",
	})
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

func readMultipartFile(t *testing.T, r *http.Request, field string) (*multipart.FileHeader, []byte) {
	t.Helper()
	if err := r.ParseMultipartForm(10 << 20); err != nil {
		t.Fatalf("parse form: %v", err)
	}
	file, header, err := r.FormFile(field)
	if err != nil {
		t.Fatalf("read form file: %v", err)
	}
	defer func() {
		_ = file.Close()
	}()
	bytes, err := io.ReadAll(file)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	return header, bytes
}
