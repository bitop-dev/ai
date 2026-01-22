package revai

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

func TestCreateRevAIDefaults(t *testing.T) {
	client := CreateRevAI(Settings{})
	if client.baseURL != DefaultBaseURL {
		t.Fatalf("expected default base URL %q, got %q", DefaultBaseURL, client.baseURL)
	}
	if client.providerID != provider.ProviderID(DefaultProviderName) {
		t.Fatalf("expected provider ID %q, got %q", DefaultProviderName, client.providerID)
	}
}

func TestRevaiTranscriptionFlow(t *testing.T) {
	t.Setenv("REVAI_API_KEY", "test-key")
	audio := []byte("audio-bytes")
	var pollCalls int
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/speechtotext/v1/jobs":
			if r.Method != http.MethodPost {
				t.Fatalf("unexpected method: %s", r.Method)
			}
			if r.Header.Get("Authorization") != "Bearer test-key" {
				t.Fatalf("missing auth header")
			}
			if r.Header.Get("X-Custom") != "custom" {
				t.Fatalf("missing custom header")
			}
			config := readMultipartConfig(t, r)
			if config["transcriber"] != "machine" {
				t.Fatalf("unexpected transcriber: %#v", config["transcriber"])
			}
			if config["metadata"] != "job-123" {
				t.Fatalf("unexpected metadata: %#v", config["metadata"])
			}
			notification, ok := config["notification_config"].(map[string]any)
			if !ok || notification["url"] != "https://example.com/hook" {
				t.Fatalf("unexpected notification_config: %#v", config["notification_config"])
			}
			authHeaders, ok := notification["auth_headers"].(map[string]any)
			if !ok || authHeaders["Authorization"] != "Bearer token" {
				t.Fatalf("unexpected auth headers: %#v", notification["auth_headers"])
			}
			if config["skip_punctuation"] != true {
				t.Fatalf("unexpected skip_punctuation: %#v", config["skip_punctuation"])
			}
			if config["remove_disfluencies"] != true {
				t.Fatalf("unexpected remove_disfluencies: %#v", config["remove_disfluencies"])
			}
			if config["language"] != "en" {
				t.Fatalf("unexpected language: %#v", config["language"])
			}
			segments, ok := config["segments_to_transcribe"].([]any)
			if !ok || len(segments) != 1 {
				t.Fatalf("unexpected segments_to_transcribe: %#v", config["segments_to_transcribe"])
			}
			speakerNames, ok := config["speaker_names"].([]any)
			if !ok || len(speakerNames) != 1 {
				t.Fatalf("unexpected speaker_names: %#v", config["speaker_names"])
			}
			nameEntry, ok := speakerNames[0].(map[string]any)
			if !ok || nameEntry["display_name"] != "Agent" {
				t.Fatalf("unexpected speaker_names entry: %#v", speakerNames[0])
			}
			fileHeader, fileBytes := readMultipartFile(t, r, "media")
			if fileHeader.Filename != "audio.wav" {
				t.Fatalf("unexpected file name: %s", fileHeader.Filename)
			}
			if fileHeader.Header.Get("Content-Type") != "audio/wav" {
				t.Fatalf("unexpected content type: %s", fileHeader.Header.Get("Content-Type"))
			}
			if string(fileBytes) != string(audio) {
				t.Fatalf("unexpected audio payload")
			}
			w.WriteHeader(http.StatusOK)
			fmt.Fprint(w, `{"id":"job-1","status":"in_progress","language":"en"}`)
		case "/speechtotext/v1/jobs/job-1":
			pollCalls++
			w.WriteHeader(http.StatusOK)
			if pollCalls < 2 {
				fmt.Fprint(w, `{"id":"job-1","status":"in_progress"}`)
				return
			}
			fmt.Fprint(w, `{"id":"job-1","status":"transcribed"}`)
		case "/speechtotext/v1/jobs/job-1/transcript":
			w.WriteHeader(http.StatusOK)
			fmt.Fprint(w, `{"monologues":[{"elements":[{"type":"text","value":"hello ","ts":0.1,"end_ts":0.5},{"type":"text","value":"world","ts":0.5,"end_ts":1.0}]}]}`)
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	client := CreateRevAI(Settings{
		BaseURL:      server.URL,
		Headers:      map[string]string{"X-Custom": "custom"},
		PollInterval: time.Millisecond,
		PollTimeout:  50 * time.Millisecond,
	})
	model, err := client.TranscriptionModel("machine")
	if err != nil {
		t.Fatalf("transcription model: %v", err)
	}
	_, err = model.DoGenerate(context.Background(), provider.TranscriptionModelV3CallOptions{
		Audio:     audio,
		MediaType: "audio/wav",
		RequestOptions: provider.RequestOptions{
			ProviderOptions: provider.ProviderOptions{
				"revai": provider.JSONObject{
					"metadata": "job-123",
					"notificationConfig": provider.JSONObject{
						"url": "https://example.com/hook",
						"authHeaders": provider.JSONObject{
							"Authorization": "Bearer token",
						},
					},
					"deleteAfterSeconds": 120,
					"skipPunctuation":    true,
					"removeDisfluencies": true,
					"segmentsToTranscribe": []provider.JSONObject{
						{
							"start": 0.1,
							"end":   1.2,
						},
					},
					"speakerNames": []provider.JSONObject{
						{
							"displayName": "Agent",
						},
					},
					"language": "en",
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

func TestRevaiAuthError(t *testing.T) {
	t.Setenv("REVAI_API_KEY", "test-key")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		fmt.Fprint(w, `{"error":{"message":"bad token","code":401}}`)
	}))
	defer server.Close()

	client := CreateRevAI(Settings{BaseURL: server.URL})
	model, err := client.TranscriptionModel("machine")
	if err != nil {
		t.Fatalf("transcription model: %v", err)
	}
	_, err = model.DoGenerate(context.Background(), provider.TranscriptionModelV3CallOptions{
		Audio:     []byte("audio"),
		MediaType: "audio/wav",
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

func readMultipartConfig(t *testing.T, r *http.Request) map[string]any {
	t.Helper()
	if err := r.ParseMultipartForm(10 << 20); err != nil {
		t.Fatalf("parse form: %v", err)
	}
	configField := r.FormValue("config")
	if configField == "" {
		t.Fatalf("missing config field")
	}
	var config map[string]any
	if err := json.Unmarshal([]byte(configField), &config); err != nil {
		t.Fatalf("decode config: %v", err)
	}
	return config
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
