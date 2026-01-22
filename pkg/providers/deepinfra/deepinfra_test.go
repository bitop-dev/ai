package deepinfra

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/vercel/ai-sdk-go/pkg/provider"
)

func TestCreateDeepInfraDefaults(t *testing.T) {
	client := CreateDeepInfra(Settings{})
	if client.baseURL != DefaultBaseURL {
		t.Fatalf("expected default base URL %q, got %q", DefaultBaseURL, client.baseURL)
	}
	if client.providerID != provider.ProviderID(DefaultProviderName) {
		t.Fatalf("expected provider ID %q, got %q", DefaultProviderName, client.providerID)
	}
}

func TestDeepInfraLanguageModelUsesOpenAIBaseURL(t *testing.T) {
	t.Setenv("DEEPINFRA_API_KEY", "test-key")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/openai/chat/completions" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Fatalf("missing auth header")
		}
		if r.Header.Get("X-Custom") != "custom" {
			t.Fatalf("missing custom header")
		}

		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `{}`)
	}))
	defer server.Close()

	client := CreateDeepInfra(Settings{
		BaseURL: server.URL,
		Headers: map[string]string{"X-Custom": "custom"},
	})
	model, err := client.LanguageModel("meta-llama/Llama-3.1-8B-Instruct")
	if err != nil {
		t.Fatalf("language model: %v", err)
	}
	_, err = model.DoGenerate(context.Background(), provider.LanguageModelV3CallOptions{
		Prompt: provider.Prompt{Messages: []provider.ModelMessage{
			{Role: provider.RoleUser, Content: []provider.ContentPart{provider.TextContent{Text: "Hello"}}},
		}},
	})
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
}

func TestDeepInfraImageModelInferencePayload(t *testing.T) {
	t.Setenv("DEEPINFRA_API_KEY", "test-key")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/inference/stabilityai/sd3.5" {
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
		if payload["aspect_ratio"] != "16:9" {
			t.Fatalf("unexpected aspect_ratio: %#v", payload["aspect_ratio"])
		}
		if payload["width"] != "1024" || payload["height"] != "768" {
			t.Fatalf("unexpected size: %#v", payload)
		}
		if payload["seed"] != float64(123) {
			t.Fatalf("unexpected seed: %#v", payload["seed"])
		}
		if payload["num_inference_steps"] != float64(30) {
			t.Fatalf("unexpected provider override: %#v", payload["num_inference_steps"])
		}

		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `{}`)
	}))
	defer server.Close()

	client := CreateDeepInfra(Settings{
		BaseURL: server.URL,
		Headers: map[string]string{"X-Custom": "custom"},
	})
	model, err := client.ImageModel("stabilityai/sd3.5")
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
				"deepinfra": provider.JSONObject{
					"num_inference_steps": 30,
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
}
