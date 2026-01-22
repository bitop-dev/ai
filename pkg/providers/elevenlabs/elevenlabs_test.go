package elevenlabs

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/bitop-dev/ai/pkg/provider"
)

func TestCreateElevenLabsDefaults(t *testing.T) {
	client := CreateElevenLabs(Settings{})
	if client.baseURL != DefaultBaseURL {
		t.Fatalf("expected default base URL %q, got %q", DefaultBaseURL, client.baseURL)
	}
	if client.providerID != provider.ProviderID(DefaultProviderName) {
		t.Fatalf("expected provider ID %q, got %q", DefaultProviderName, client.providerID)
	}
}

func TestElevenLabsSpeechModelRequest(t *testing.T) {
	t.Setenv("ELEVENLABS_API_KEY", "test-key")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/text-to-speech/voice-123" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if r.Header.Get("xi-api-key") != "test-key" {
			t.Fatalf("missing auth header")
		}
		if r.Header.Get("X-Custom") != "custom" {
			t.Fatalf("missing custom header")
		}
		query := r.URL.Query()
		if query.Get("output_format") != "mp3_44100_128" {
			t.Fatalf("unexpected output format: %s", query.Get("output_format"))
		}
		if query.Get("enable_logging") != "true" {
			t.Fatalf("unexpected enable_logging: %s", query.Get("enable_logging"))
		}
		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode payload: %v", err)
		}
		if payload["text"] != "Hello" {
			t.Fatalf("unexpected text: %#v", payload["text"])
		}
		if payload["model_id"] != "eleven_multilingual_v2" {
			t.Fatalf("unexpected model: %#v", payload["model_id"])
		}
		if payload["language_code"] != "en" {
			t.Fatalf("unexpected language_code: %#v", payload["language_code"])
		}
		voiceSettings, ok := payload["voice_settings"].(map[string]any)
		if !ok {
			t.Fatalf("missing voice_settings")
		}
		if voiceSettings["stability"] != 0.5 {
			t.Fatalf("unexpected stability: %#v", voiceSettings["stability"])
		}
		if voiceSettings["similarity_boost"] != 0.7 {
			t.Fatalf("unexpected similarity_boost: %#v", voiceSettings["similarity_boost"])
		}
		if voiceSettings["style"] != 0.4 {
			t.Fatalf("unexpected style: %#v", voiceSettings["style"])
		}
		if voiceSettings["use_speaker_boost"] != true {
			t.Fatalf("unexpected use_speaker_boost: %#v", voiceSettings["use_speaker_boost"])
		}
		if voiceSettings["speed"] != 1.25 {
			t.Fatalf("unexpected speed: %#v", voiceSettings["speed"])
		}
		locators, ok := payload["pronunciation_dictionary_locators"].([]any)
		if !ok || len(locators) != 1 {
			t.Fatalf("unexpected pronunciation locators: %#v", payload["pronunciation_dictionary_locators"])
		}
		locator, ok := locators[0].(map[string]any)
		if !ok {
			t.Fatalf("unexpected locator type")
		}
		if locator["pronunciation_dictionary_id"] != "dict-1" {
			t.Fatalf("unexpected dictionary id: %#v", locator["pronunciation_dictionary_id"])
		}
		if locator["version_id"] != "ver-1" {
			t.Fatalf("unexpected version id: %#v", locator["version_id"])
		}
		if payload["seed"] != 99.0 {
			t.Fatalf("unexpected seed: %#v", payload["seed"])
		}
		if payload["previous_text"] != "prev" {
			t.Fatalf("unexpected previous_text: %#v", payload["previous_text"])
		}
		if payload["next_text"] != "next" {
			t.Fatalf("unexpected next_text: %#v", payload["next_text"])
		}
		if !stringSliceContains(payload["previous_request_ids"], []string{"req1"}) {
			t.Fatalf("unexpected previous_request_ids: %#v", payload["previous_request_ids"])
		}
		if !stringSliceContains(payload["next_request_ids"], []string{"req2", "req3"}) {
			t.Fatalf("unexpected next_request_ids: %#v", payload["next_request_ids"])
		}
		if payload["apply_text_normalization"] != "on" {
			t.Fatalf("unexpected apply_text_normalization: %#v", payload["apply_text_normalization"])
		}
		if payload["apply_language_text_normalization"] != true {
			t.Fatalf("unexpected apply_language_text_normalization: %#v", payload["apply_language_text_normalization"])
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("audio"))
	}))
	defer server.Close()

	client := CreateElevenLabs(Settings{
		BaseURL: server.URL,
		Headers: map[string]string{"X-Custom": "custom"},
	})
	model, err := client.SpeechModel("eleven_multilingual_v2")
	if err != nil {
		t.Fatalf("speech model: %v", err)
	}
	_, err = model.DoGenerate(context.Background(), provider.SpeechModelV3CallOptions{
		Text:         "Hello",
		Voice:        "voice-123",
		OutputFormat: "mp3",
		Speed:        1.25,
		Language:     "en",
		RequestOptions: provider.RequestOptions{
			ProviderOptions: provider.ProviderOptions{
				"elevenlabs": provider.JSONObject{
					"voiceSettings": provider.JSONObject{
						"stability":       0.5,
						"similarityBoost": 0.7,
						"style":           0.4,
						"useSpeakerBoost": true,
					},
					"pronunciationDictionaryLocators": []provider.JSONObject{
						{
							"pronunciationDictionaryId": "dict-1",
							"versionId":                 "ver-1",
						},
					},
					"seed":                           99,
					"previousText":                   "prev",
					"nextText":                       "next",
					"previousRequestIds":             []string{"req1"},
					"nextRequestIds":                 []string{"req2", "req3"},
					"applyTextNormalization":         "on",
					"applyLanguageTextNormalization": true,
					"enableLogging":                  true,
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
}

func TestElevenLabsTranscriptionModelRequest(t *testing.T) {
	t.Setenv("ELEVENLABS_API_KEY", "test-key")
	audio := []byte("audio-bytes")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/speech-to-text" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if r.Header.Get("xi-api-key") != "test-key" {
			t.Fatalf("missing auth header")
		}
		if err := r.ParseMultipartForm(1 << 20); err != nil {
			t.Fatalf("parse multipart: %v", err)
		}
		if r.FormValue("model_id") != "scribe_v1" {
			t.Fatalf("unexpected model: %s", r.FormValue("model_id"))
		}
		if r.FormValue("diarize") != "false" {
			t.Fatalf("unexpected diarize: %s", r.FormValue("diarize"))
		}
		if r.FormValue("language_code") != "fr" {
			t.Fatalf("unexpected language_code: %s", r.FormValue("language_code"))
		}
		if r.FormValue("tag_audio_events") != "false" {
			t.Fatalf("unexpected tag_audio_events: %s", r.FormValue("tag_audio_events"))
		}
		if r.FormValue("num_speakers") != "2" {
			t.Fatalf("unexpected num_speakers: %s", r.FormValue("num_speakers"))
		}
		if r.FormValue("timestamps_granularity") != "word" {
			t.Fatalf("unexpected timestamps_granularity: %s", r.FormValue("timestamps_granularity"))
		}
		if r.FormValue("file_format") != "pcm_s16le_16" {
			t.Fatalf("unexpected file_format: %s", r.FormValue("file_format"))
		}
		file, header, err := r.FormFile("file")
		if err != nil {
			t.Fatalf("file upload: %v", err)
		}
		defer file.Close()
		if header.Filename != "audio.wav" {
			t.Fatalf("unexpected filename: %s", header.Filename)
		}
		body, err := io.ReadAll(file)
		if err != nil {
			t.Fatalf("read body: %v", err)
		}
		if !bytes.Equal(body, audio) {
			t.Fatalf("unexpected audio payload")
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"language_code":"fr","text":"hi"}`))
	}))
	defer server.Close()

	client := CreateElevenLabs(Settings{BaseURL: server.URL})
	model, err := client.TranscriptionModel("scribe_v1")
	if err != nil {
		t.Fatalf("transcription model: %v", err)
	}
	_, err = model.DoGenerate(context.Background(), provider.TranscriptionModelV3CallOptions{
		Audio:     audio,
		MediaType: "audio/x-wav",
		RequestOptions: provider.RequestOptions{
			ProviderOptions: provider.ProviderOptions{
				"elevenlabs": provider.JSONObject{
					"languageCode":          "fr",
					"tagAudioEvents":        false,
					"numSpeakers":           2,
					"timestampsGranularity": "word",
					"diarize":               false,
					"fileFormat":            "pcm_s16le_16",
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("transcribe: %v", err)
	}
}

func TestElevenLabsAuthError(t *testing.T) {
	t.Setenv("ELEVENLABS_API_KEY", "test-key")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":{"message":"bad token","code":401}}`))
	}))
	defer server.Close()

	client := CreateElevenLabs(Settings{BaseURL: server.URL})
	model, err := client.SpeechModel("eleven_multilingual_v2")
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

func stringSliceContains(value any, expected []string) bool {
	items, ok := value.([]any)
	if !ok {
		return false
	}
	if len(items) != len(expected) {
		return false
	}
	for i, item := range items {
		if item != expected[i] {
			return false
		}
	}
	return true
}
