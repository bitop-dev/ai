package replicate

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

func TestCreateReplicateDefaults(t *testing.T) {
	client := CreateReplicate(Settings{})
	if client.baseURL != DefaultBaseURL {
		t.Fatalf("expected default base URL %q, got %q", DefaultBaseURL, client.baseURL)
	}
	if client.providerID != provider.ProviderID(DefaultProviderName) {
		t.Fatalf("expected provider ID %q, got %q", DefaultProviderName, client.providerID)
	}
}

func TestReplicateImageModelPayload(t *testing.T) {
	t.Setenv("REPLICATE_API_TOKEN", "test-key")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/models/black-forest-labs/flux-1.1-pro/predictions" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Fatalf("missing auth header")
		}
		if r.Header.Get("X-Custom") != "custom" {
			t.Fatalf("missing custom header")
		}
		if r.Header.Get("Prefer") != "wait=120" {
			t.Fatalf("unexpected prefer header: %s", r.Header.Get("Prefer"))
		}
		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode payload: %v", err)
		}
		input, ok := payload["input"].(map[string]any)
		if !ok {
			t.Fatalf("missing input: %#v", payload["input"])
		}
		if input["prompt"] != "A neon skyline" {
			t.Fatalf("unexpected prompt: %#v", input["prompt"])
		}
		if input["num_outputs"] != float64(2) {
			t.Fatalf("unexpected num_outputs: %#v", input["num_outputs"])
		}
		if input["aspect_ratio"] != "16:9" {
			t.Fatalf("unexpected aspect_ratio: %#v", input["aspect_ratio"])
		}
		if input["size"] != "1024x768" {
			t.Fatalf("unexpected size: %#v", input["size"])
		}
		if input["seed"] != float64(123) {
			t.Fatalf("unexpected seed: %#v", input["seed"])
		}
		if input["num_inference_steps"] != float64(30) {
			t.Fatalf("unexpected provider option: %#v", input["num_inference_steps"])
		}
		if _, ok := input["maxWaitTimeInSeconds"]; ok {
			t.Fatalf("maxWaitTimeInSeconds should not be in input")
		}
		if payload["webhook"] != "https://example.com" {
			t.Fatalf("unexpected webhook: %#v", payload["webhook"])
		}

		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `{"id":"pred-1","status":"succeeded","output":"https://example.com/image.png"}`)
	}))
	defer server.Close()

	client := CreateReplicate(Settings{
		BaseURL: server.URL,
		Headers: map[string]string{"X-Custom": "custom"},
	})
	model, err := client.ImageModel("black-forest-labs/flux-1.1-pro")
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
				"replicate": provider.JSONObject{
					"maxWaitTimeInSeconds": 120,
					"num_inference_steps":  30,
					"request": provider.JSONObject{
						"webhook": "https://example.com",
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
}

func TestReplicateImageModelPollsPrediction(t *testing.T) {
	t.Setenv("REPLICATE_API_TOKEN", "test-key")
	var postCalls int
	var getCalls int
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost:
			postCalls++
			if r.URL.Path != "/predictions" {
				t.Fatalf("unexpected path: %s", r.URL.Path)
			}
			var payload map[string]any
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Fatalf("decode payload: %v", err)
			}
			if payload["version"] != "abc123" {
				t.Fatalf("missing version: %#v", payload["version"])
			}
			w.WriteHeader(http.StatusOK)
			fmt.Fprintf(w, `{"id":"pred-1","status":"processing","urls":{"get":"%s/predictions/pred-1"}}`, server.URL)
		case http.MethodGet:
			getCalls++
			if r.URL.Path != "/predictions/pred-1" {
				t.Fatalf("unexpected path: %s", r.URL.Path)
			}
			w.WriteHeader(http.StatusOK)
			fmt.Fprint(w, `{"id":"pred-1","status":"succeeded","output":["data:image/png;base64,abc"]}`)
		default:
			t.Fatalf("unexpected method: %s", r.Method)
		}
	}))
	defer server.Close()

	client := CreateReplicate(Settings{BaseURL: server.URL})
	model, err := client.ImageModel("black-forest-labs/flux-1.1-pro:abc123")
	if err != nil {
		t.Fatalf("image model: %v", err)
	}
	_, err = model.DoGenerate(context.Background(), provider.ImageModelV3CallOptions{
		Prompt: "Hello",
	})
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	if postCalls != 1 {
		t.Fatalf("expected 1 post call, got %d", postCalls)
	}
	if getCalls != 1 {
		t.Fatalf("expected 1 get call, got %d", getCalls)
	}
}

func TestReplicateImageModelAuthError(t *testing.T) {
	t.Setenv("REPLICATE_API_TOKEN", "test-key")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		fmt.Fprint(w, `{"detail":"bad token"}`)
	}))
	defer server.Close()

	client := CreateReplicate(Settings{BaseURL: server.URL})
	model, err := client.ImageModel("black-forest-labs/flux-1.1-pro")
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
