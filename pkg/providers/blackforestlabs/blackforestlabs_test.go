package blackforestlabs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/vercel/ai-sdk-go/pkg/provider"
)

func TestCreateBlackForestLabsDefaults(t *testing.T) {
	client := CreateBlackForestLabs(Settings{})
	if client.baseURL != DefaultBaseURL {
		t.Fatalf("expected default base URL %q, got %q", DefaultBaseURL, client.baseURL)
	}
	if client.providerID != provider.ProviderID(DefaultProviderName) {
		t.Fatalf("expected provider ID %q, got %q", DefaultProviderName, client.providerID)
	}
}

func TestBlackForestLabsImageModelPayload(t *testing.T) {
	t.Setenv("BFL_API_KEY", "test-key")
	var pollCalls int
	var imageCalls int
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/flux-pro-1.1":
			if r.Method != http.MethodPost {
				t.Fatalf("unexpected method: %s", r.Method)
			}
			if r.Header.Get("x-key") != "test-key" {
				t.Fatalf("missing auth header")
			}
			if r.Header.Get("X-Custom") != "custom" {
				t.Fatalf("missing custom header")
			}
			var payload map[string]any
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Fatalf("decode payload: %v", err)
			}
			if payload["prompt"] != "A neon skyline" {
				t.Fatalf("unexpected prompt: %#v", payload["prompt"])
			}
			if payload["seed"] != float64(42) {
				t.Fatalf("unexpected seed: %#v", payload["seed"])
			}
			if payload["aspect_ratio"] != "4:3" {
				t.Fatalf("unexpected aspect_ratio: %#v", payload["aspect_ratio"])
			}
			if payload["width"] != float64(512) || payload["height"] != float64(512) {
				t.Fatalf("unexpected size: %#v", payload)
			}
			if payload["steps"] != float64(30) {
				t.Fatalf("unexpected steps: %#v", payload["steps"])
			}
			if payload["guidance"] != float64(3.5) {
				t.Fatalf("unexpected guidance: %#v", payload["guidance"])
			}
			if payload["image_prompt"] != "sketch" {
				t.Fatalf("unexpected image_prompt: %#v", payload["image_prompt"])
			}
			if payload["image_prompt_strength"] != float64(0.6) {
				t.Fatalf("unexpected image_prompt_strength: %#v", payload["image_prompt_strength"])
			}
			if payload["input_image"] != "aW1n" {
				t.Fatalf("unexpected input_image: %#v", payload["input_image"])
			}
			if payload["input_image_2"] != "https://example.com/ref.png" {
				t.Fatalf("unexpected input_image_2: %#v", payload["input_image_2"])
			}
			if payload["mask"] != "bWFzaw==" {
				t.Fatalf("unexpected mask: %#v", payload["mask"])
			}
			if payload["output_format"] != "png" {
				t.Fatalf("unexpected output_format: %#v", payload["output_format"])
			}
			if payload["prompt_upsampling"] != true {
				t.Fatalf("unexpected prompt_upsampling: %#v", payload["prompt_upsampling"])
			}
			if payload["raw"] != true {
				t.Fatalf("unexpected raw: %#v", payload["raw"])
			}
			if payload["safety_tolerance"] != float64(2) {
				t.Fatalf("unexpected safety_tolerance: %#v", payload["safety_tolerance"])
			}
			if payload["webhook_secret"] != "secret" {
				t.Fatalf("unexpected webhook_secret: %#v", payload["webhook_secret"])
			}
			if payload["webhook_url"] != "https://example.com/hook" {
				t.Fatalf("unexpected webhook_url: %#v", payload["webhook_url"])
			}
			if _, ok := payload["pollIntervalMillis"]; ok {
				t.Fatalf("pollIntervalMillis should not be in payload")
			}
			if _, ok := payload["pollTimeoutMillis"]; ok {
				t.Fatalf("pollTimeoutMillis should not be in payload")
			}
			w.WriteHeader(http.StatusOK)
			fmt.Fprintf(w, `{"id":"req-1","polling_url":"%s/poll"}`, server.URL)
		case "/poll":
			pollCalls++
			if r.Header.Get("x-key") != "test-key" {
				t.Fatalf("missing poll auth header")
			}
			if r.URL.Query().Get("id") != "req-1" {
				t.Fatalf("missing poll id: %s", r.URL.RawQuery)
			}
			if pollCalls == 1 {
				w.WriteHeader(http.StatusOK)
				fmt.Fprint(w, `{"status":"Pending"}`)
				return
			}
			w.WriteHeader(http.StatusOK)
			fmt.Fprintf(w, `{"status":"Ready","result":{"sample":"%s/image"}}`, server.URL)
		case "/image":
			imageCalls++
			if r.Header.Get("x-key") != "test-key" {
				t.Fatalf("missing image auth header")
			}
			w.WriteHeader(http.StatusOK)
			fmt.Fprint(w, "image-bytes")
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	client := CreateBlackForestLabs(Settings{
		BaseURL:      server.URL,
		Headers:      map[string]string{"X-Custom": "custom"},
		PollInterval: time.Millisecond,
		PollTimeout:  time.Second,
	})
	model, err := client.ImageModel("flux-pro-1.1")
	if err != nil {
		t.Fatalf("image model: %v", err)
	}
	_, err = model.DoGenerate(context.Background(), provider.ImageModelV3CallOptions{
		Prompt: "A neon skyline",
		Size:   "1024x768",
		Seed:   42,
		RequestOptions: provider.RequestOptions{
			ProviderOptions: provider.ProviderOptions{
				"black-forest-labs": provider.JSONObject{
					"width":               512,
					"height":              512,
					"steps":               30,
					"guidance":            3.5,
					"imagePrompt":         "sketch",
					"imagePromptStrength": 0.6,
					"inputImage": provider.ImageContent{
						Data: []byte("img"),
					},
					"inputImage2": "https://example.com/ref.png",
					"mask": provider.FileContent{
						Data: []byte("mask"),
					},
					"outputFormat":       "png",
					"promptUpsampling":   true,
					"raw":                true,
					"safetyTolerance":    2,
					"webhookSecret":      "secret",
					"webhookUrl":         "https://example.com/hook",
					"pollIntervalMillis": 1,
					"pollTimeoutMillis":  250,
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	if pollCalls != 2 {
		t.Fatalf("expected 2 poll calls, got %d", pollCalls)
	}
	if imageCalls != 1 {
		t.Fatalf("expected 1 image call, got %d", imageCalls)
	}
}

func TestBlackForestLabsImageModelBase64Sample(t *testing.T) {
	t.Setenv("BFL_API_KEY", "test-key")
	var pollCalls int
	var imageCalls int
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/flux-pro-1.1":
			w.WriteHeader(http.StatusOK)
			fmt.Fprintf(w, `{"id":"req-1","polling_url":"%s/poll"}`, server.URL)
		case "/poll":
			pollCalls++
			w.WriteHeader(http.StatusOK)
			fmt.Fprint(w, `{"status":"Ready","result":{"sample":"aW1n"}}`)
		case "/image":
			imageCalls++
			w.WriteHeader(http.StatusOK)
			fmt.Fprint(w, "image-bytes")
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	client := CreateBlackForestLabs(Settings{BaseURL: server.URL, PollInterval: time.Millisecond})
	model, err := client.ImageModel("flux-pro-1.1")
	if err != nil {
		t.Fatalf("image model: %v", err)
	}
	_, err = model.DoGenerate(context.Background(), provider.ImageModelV3CallOptions{Prompt: "Hello"})
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	if pollCalls != 1 {
		t.Fatalf("expected 1 poll call, got %d", pollCalls)
	}
	if imageCalls != 0 {
		t.Fatalf("expected no image calls, got %d", imageCalls)
	}
}

func TestBlackForestLabsImageModelAuthError(t *testing.T) {
	t.Setenv("BFL_API_KEY", "test-key")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		fmt.Fprint(w, `{"message":"bad token"}`)
	}))
	defer server.Close()

	client := CreateBlackForestLabs(Settings{BaseURL: server.URL})
	model, err := client.ImageModel("flux-pro-1.1")
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
