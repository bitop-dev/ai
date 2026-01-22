package fal

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/vercel/ai-sdk-go/pkg/provider"
)

func TestCreateFalDefaults(t *testing.T) {
	client := CreateFal(Settings{})
	if client.baseURL != DefaultBaseURL {
		t.Fatalf("expected default base URL %q, got %q", DefaultBaseURL, client.baseURL)
	}
	if client.queueURL != DefaultQueueURL {
		t.Fatalf("expected default queue URL %q, got %q", DefaultQueueURL, client.queueURL)
	}
	if client.providerID != provider.ProviderID(DefaultProviderName) {
		t.Fatalf("expected provider ID %q, got %q", DefaultProviderName, client.providerID)
	}
}

func TestFalImageModelPayload(t *testing.T) {
	t.Setenv("FAL_API_KEY", "test-key")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/fal-ai/flux" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Key test-key" {
			t.Fatalf("missing auth header")
		}
		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode payload: %v", err)
		}
		if payload["prompt"] != "A neon skyline" {
			t.Fatalf("unexpected prompt: %#v", payload["prompt"])
		}
		if payload["num_images"] != float64(2) {
			t.Fatalf("unexpected num_images: %#v", payload["num_images"])
		}
		if payload["seed"] != float64(123) {
			t.Fatalf("unexpected seed: %#v", payload["seed"])
		}
		imageSize, ok := payload["image_size"].(map[string]any)
		if !ok {
			t.Fatalf("expected image_size map, got %#v", payload["image_size"])
		}
		if imageSize["width"] != float64(1024) || imageSize["height"] != float64(768) {
			t.Fatalf("unexpected image_size: %#v", imageSize)
		}
		if payload["guidance_scale"] != float64(7.5) {
			t.Fatalf("unexpected guidance_scale: %#v", payload["guidance_scale"])
		}
		if payload["image_url"] != "data:image/png;base64,aW1n" {
			t.Fatalf("unexpected image_url: %#v", payload["image_url"])
		}
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `{"images":[{"url":"https://example.com/image.png"}]}`)
	}))
	defer server.Close()

	client := CreateFal(Settings{BaseURL: server.URL})
	model, err := client.ImageModel("fal-ai/flux")
	if err != nil {
		t.Fatalf("image model: %v", err)
	}
	_, err = model.DoGenerate(context.Background(), provider.ImageModelV3CallOptions{
		Prompt: "A neon skyline",
		N:      2,
		Size:   "1024x768",
		Seed:   123,
		RequestOptions: provider.RequestOptions{
			ProviderOptions: provider.ProviderOptions{
				"fal": provider.JSONObject{
					"guidanceScale": 7.5,
					"imageUrl": provider.ImageContent{
						MediaType: "image/png",
						Data:      []byte("img"),
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
}

func TestFalImageModelAuthError(t *testing.T) {
	t.Setenv("FAL_API_KEY", "test-key")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		fmt.Fprint(w, `{"error":{"message":"bad token","code":401}}`)
	}))
	defer server.Close()

	client := CreateFal(Settings{BaseURL: server.URL})
	model, err := client.ImageModel("fal-ai/flux")
	if err != nil {
		t.Fatalf("image model: %v", err)
	}
	_, err = model.DoGenerate(context.Background(), provider.ImageModelV3CallOptions{Prompt: "Hello"})
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

func TestFalTranscriptionModelPolling(t *testing.T) {
	t.Setenv("FAL_API_KEY", "test-key")
	var pollCalls int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/fal-ai/wizper":
			if r.Header.Get("Authorization") != "Key test-key" {
				t.Fatalf("missing auth header")
			}
			var payload map[string]any
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Fatalf("decode payload: %v", err)
			}
			if payload["task"] != "transcribe" {
				t.Fatalf("unexpected task: %#v", payload["task"])
			}
			if payload["audio_url"] != "data:audio/wav;base64,ZGF0YQ==" {
				t.Fatalf("unexpected audio_url: %#v", payload["audio_url"])
			}
			w.WriteHeader(http.StatusOK)
			fmt.Fprint(w, `{"request_id":"test-id"}`)
		case r.Method == http.MethodGet && r.URL.Path == "/fal-ai/wizper/requests/test-id":
			pollCalls++
			if pollCalls == 1 {
				w.WriteHeader(http.StatusBadRequest)
				fmt.Fprint(w, `{"detail":"Request is still in progress"}`)
				return
			}
			w.WriteHeader(http.StatusOK)
			fmt.Fprint(w, `{"text":"hello"}`)
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	client := CreateFal(Settings{QueueURL: server.URL})
	model, err := client.TranscriptionModel("wizper")
	if err != nil {
		t.Fatalf("transcription model: %v", err)
	}
	_, err = model.DoGenerate(context.Background(), provider.TranscriptionModelV3CallOptions{
		Audio:     []byte("data"),
		MediaType: "audio/wav",
	})
	if err != nil {
		t.Fatalf("transcribe: %v", err)
	}
	if pollCalls != 2 {
		t.Fatalf("expected 2 poll calls, got %d", pollCalls)
	}
}
