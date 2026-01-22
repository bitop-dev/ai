package baseten

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/bitop-dev/ai/pkg/provider"
)

func TestCreateBasetenDefaults(t *testing.T) {
	client := CreateBaseten(Settings{})
	if client.baseURL != DefaultBaseURL {
		t.Fatalf("expected default base URL %q, got %q", DefaultBaseURL, client.baseURL)
	}
	if client.providerID != provider.ProviderID(DefaultProviderName) {
		t.Fatalf("expected provider ID %q, got %q", DefaultProviderName, client.providerID)
	}
}

func TestBasetenImageModelPayload(t *testing.T) {
	t.Setenv("BASETEN_API_KEY", "test-key")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/models/flux-1/predict" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
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
		if payload["num_images"] != float64(2) {
			t.Fatalf("unexpected num_images: %#v", payload["num_images"])
		}
		if payload["width"] != float64(1024) || payload["height"] != float64(768) {
			t.Fatalf("unexpected size: %#v", payload)
		}
		if payload["seed"] != float64(123) {
			t.Fatalf("unexpected seed: %#v", payload["seed"])
		}
		if payload["aspect_ratio"] != "16:9" {
			t.Fatalf("unexpected aspect_ratio: %#v", payload["aspect_ratio"])
		}
		if payload["guidance_scale"] != float64(7.5) {
			t.Fatalf("unexpected guidance_scale: %#v", payload["guidance_scale"])
		}
		if payload["negative_prompt"] != "fog" {
			t.Fatalf("unexpected negative_prompt: %#v", payload["negative_prompt"])
		}
		if payload["metadata"] != "trace" {
			t.Fatalf("unexpected metadata: %#v", payload["metadata"])
		}

		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `{"images":[{"url":"https://example.com/image.png"}]}`)
	}))
	defer server.Close()

	client := CreateBaseten(Settings{
		BaseURL: server.URL,
		Headers: map[string]string{"X-Custom": "custom"},
	})
	model, err := client.ImageModel("flux-1")
	if err != nil {
		t.Fatalf("image model: %v", err)
	}
	_, err = model.DoGenerate(context.Background(), provider.ImageModelV3CallOptions{
		Prompt:      "A neon skyline",
		N:           2,
		Size:        "1024x768",
		AspectRatio: "16:9",
		Seed:        123,
		RequestOptions: provider.RequestOptions{
			ProviderOptions: provider.ProviderOptions{
				"baseten": provider.JSONObject{
					"guidanceScale":  7.5,
					"negativePrompt": "fog",
					"request": provider.JSONObject{
						"metadata": "trace",
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
}

func TestBasetenImageModelAuthError(t *testing.T) {
	t.Setenv("BASETEN_API_KEY", "test-key")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		fmt.Fprint(w, `{"error":"bad token"}`)
	}))
	defer server.Close()

	client := CreateBaseten(Settings{BaseURL: server.URL})
	model, err := client.ImageModel("flux-1")
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
