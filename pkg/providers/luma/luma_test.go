package luma

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

func TestCreateLumaDefaults(t *testing.T) {
	client := CreateLuma(Settings{})
	if client.baseURL != DefaultBaseURL {
		t.Fatalf("expected default base URL %q, got %q", DefaultBaseURL, client.baseURL)
	}
	if client.providerID != provider.ProviderID(DefaultProviderName) {
		t.Fatalf("expected provider ID %q, got %q", DefaultProviderName, client.providerID)
	}
}

func TestLumaImageModelPayloadAndPolling(t *testing.T) {
	t.Setenv("LUMA_API_KEY", "test-key")
	var pollCalls int
	var imageCalls int
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/dream-machine/v1/generations/image":
			if r.Method != http.MethodPost {
				t.Fatalf("unexpected method: %s", r.Method)
			}
			if r.Header.Get("Authorization") != "Bearer test-key" {
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
			if payload["aspect_ratio"] != "16:9" {
				t.Fatalf("unexpected aspect_ratio: %#v", payload["aspect_ratio"])
			}
			if payload["model"] != "test-model" {
				t.Fatalf("unexpected model: %#v", payload["model"])
			}
			if payload["quality"] != "hd" {
				t.Fatalf("unexpected quality: %#v", payload["quality"])
			}
			if _, ok := payload["pollIntervalMillis"]; ok {
				t.Fatalf("pollIntervalMillis should not be in payload")
			}
			if _, ok := payload["maxPollAttempts"]; ok {
				t.Fatalf("maxPollAttempts should not be in payload")
			}
			w.WriteHeader(http.StatusOK)
			fmt.Fprint(w, `{"id":"gen-1","state":"queued"}`)
		case "/dream-machine/v1/generations/gen-1":
			pollCalls++
			if pollCalls == 1 {
				w.WriteHeader(http.StatusOK)
				fmt.Fprint(w, `{"id":"gen-1","state":"dreaming"}`)
				return
			}
			w.WriteHeader(http.StatusOK)
			fmt.Fprintf(w, `{"id":"gen-1","state":"completed","assets":{"image":"%s/image.png"}}`, server.URL)
		case "/image.png":
			imageCalls++
			w.WriteHeader(http.StatusOK)
			fmt.Fprint(w, "image-bytes")
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	client := CreateLuma(Settings{
		BaseURL:         server.URL,
		Headers:         map[string]string{"X-Custom": "custom"},
		PollInterval:    time.Millisecond,
		MaxPollAttempts: 3,
	})
	model, err := client.ImageModel("test-model")
	if err != nil {
		t.Fatalf("image model: %v", err)
	}
	_, err = model.DoGenerate(context.Background(), provider.ImageModelV3CallOptions{
		Prompt:      "A neon skyline",
		AspectRatio: "16:9",
		RequestOptions: provider.RequestOptions{
			ProviderOptions: provider.ProviderOptions{
				"luma": provider.JSONObject{
					"quality":            "hd",
					"pollIntervalMillis": 1,
					"maxPollAttempts":    2,
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

func TestLumaImageModelReferenceImagePayload(t *testing.T) {
	t.Setenv("LUMA_API_KEY", "test-key")
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/dream-machine/v1/generations/image":
			var payload map[string]any
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Fatalf("decode payload: %v", err)
			}
			refs, ok := payload["image"].([]any)
			if !ok || len(refs) != 2 {
				t.Fatalf("unexpected image refs: %#v", payload["image"])
			}
			first := refs[0].(map[string]any)
			second := refs[1].(map[string]any)
			if first["url"] != "https://example.com/ref1.png" || first["weight"] != float64(0.6) {
				t.Fatalf("unexpected first image ref: %#v", first)
			}
			if second["url"] != "https://example.com/ref2.png" || second["weight"] != float64(0.85) {
				t.Fatalf("unexpected second image ref: %#v", second)
			}
			w.WriteHeader(http.StatusOK)
			fmt.Fprintf(w, `{"id":"gen-2","state":"completed","assets":{"image":"%s/image.png"}}`, server.URL)
		case "/image.png":
			w.WriteHeader(http.StatusOK)
			fmt.Fprint(w, "image-bytes")
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	client := CreateLuma(Settings{BaseURL: server.URL})
	model, err := client.ImageModel("test-model")
	if err != nil {
		t.Fatalf("image model: %v", err)
	}
	_, err = model.DoGenerate(context.Background(), provider.ImageModelV3CallOptions{
		Prompt: "Reference image",
		RequestOptions: provider.RequestOptions{
			ProviderOptions: provider.ProviderOptions{
				"luma": provider.JSONObject{
					"files": []any{
						"https://example.com/ref1.png",
						"https://example.com/ref2.png",
					},
					"images": []any{
						provider.JSONObject{"weight": 0.6},
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
}

func TestLumaImageModelModifyImageRequiresSingleInput(t *testing.T) {
	t.Setenv("LUMA_API_KEY", "test-key")
	client := CreateLuma(Settings{BaseURL: "https://example.com"})
	model, err := client.ImageModel("test-model")
	if err != nil {
		t.Fatalf("image model: %v", err)
	}
	_, err = model.DoGenerate(context.Background(), provider.ImageModelV3CallOptions{
		Prompt: "Edit",
		RequestOptions: provider.RequestOptions{
			ProviderOptions: provider.ProviderOptions{
				"luma": provider.JSONObject{
					"referenceType": "modify_image",
					"files": []any{
						"https://example.com/ref1.png",
						"https://example.com/ref2.png",
					},
				},
			},
		},
	})
	if err == nil {
		t.Fatalf("expected error")
	}
	var invalidErr *provider.InvalidRequestError
	if !errors.As(err, &invalidErr) {
		t.Fatalf("expected invalid request error, got %T", err)
	}
}

func TestLumaImageModelAuthError(t *testing.T) {
	t.Setenv("LUMA_API_KEY", "test-key")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		fmt.Fprint(w, `{"detail":[{"msg":"bad token"}]}`)
	}))
	defer server.Close()

	client := CreateLuma(Settings{BaseURL: server.URL})
	model, err := client.ImageModel("test-model")
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
