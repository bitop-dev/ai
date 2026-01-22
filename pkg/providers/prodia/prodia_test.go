package prodia

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"testing"

	"github.com/bitop-dev/ai/pkg/provider"
)

func TestCreateProdiaDefaults(t *testing.T) {
	client := CreateProdia(Settings{})
	if client.baseURL != DefaultBaseURL {
		t.Fatalf("expected default base URL %q, got %q", DefaultBaseURL, client.baseURL)
	}
	if client.providerID != provider.ProviderID(DefaultProviderName) {
		t.Fatalf("expected provider ID %q, got %q", DefaultProviderName, client.providerID)
	}
}

func TestProdiaImageModelPayload(t *testing.T) {
	t.Setenv("PRODIA_TOKEN", "test-key")
	responseBody, contentType := createProdiaMultipartResponse(t, map[string]any{"id": "job-123"}, []byte("image"))

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/job" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Fatalf("missing auth header")
		}
		if r.Header.Get("Accept") != prodiaAcceptHeader {
			t.Fatalf("unexpected accept header: %s", r.Header.Get("Accept"))
		}

		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode payload: %v", err)
		}
		if payload["type"] != "inference.flux-fast.schnell.txt2img.v2" {
			t.Fatalf("unexpected model type: %#v", payload["type"])
		}
		config, ok := payload["config"].(map[string]any)
		if !ok {
			t.Fatalf("expected config object, got %#v", payload["config"])
		}
		if config["prompt"] != "A neon skyline" {
			t.Fatalf("unexpected prompt: %#v", config["prompt"])
		}
		if config["seed"] != float64(123) {
			t.Fatalf("unexpected seed: %#v", config["seed"])
		}
		if config["width"] != float64(512) || config["height"] != float64(512) {
			t.Fatalf("unexpected size: %#v", config)
		}
		if config["steps"] != float64(4) {
			t.Fatalf("unexpected steps: %#v", config["steps"])
		}
		if config["style_preset"] != "anime" {
			t.Fatalf("unexpected style_preset: %#v", config["style_preset"])
		}
		loras, ok := config["loras"].([]any)
		if !ok || len(loras) != 2 {
			t.Fatalf("unexpected loras: %#v", config["loras"])
		}
		if config["progressive"] != true {
			t.Fatalf("unexpected progressive: %#v", config["progressive"])
		}

		w.Header().Set("Content-Type", contentType)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(responseBody)
	}))
	defer server.Close()

	client := CreateProdia(Settings{BaseURL: server.URL})
	model, err := client.ImageModel("inference.flux-fast.schnell.txt2img.v2")
	if err != nil {
		t.Fatalf("image model: %v", err)
	}
	_, err = model.DoGenerate(context.Background(), provider.ImageModelV3CallOptions{
		Prompt: "A neon skyline",
		Size:   "1024x768",
		Seed:   123,
		RequestOptions: provider.RequestOptions{
			ProviderOptions: provider.ProviderOptions{
				"prodia": provider.JSONObject{
					"width":       512,
					"height":      512,
					"steps":       4,
					"stylePreset": "anime",
					"loras":       []string{"prodia/lora/flux/anime@v1", "prodia/lora/flux/realism@v1"},
					"progressive": true,
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
}

func TestProdiaImageModelParsesMultipartResponse(t *testing.T) {
	t.Setenv("PRODIA_TOKEN", "test-key")
	responseBody, contentType := createProdiaMultipartResponse(t, map[string]any{"id": "job-123"}, []byte("image"))

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", contentType)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(responseBody)
	}))
	defer server.Close()

	client := CreateProdia(Settings{BaseURL: server.URL})
	model, err := client.ImageModel("inference.flux-fast.schnell.txt2img.v2")
	if err != nil {
		t.Fatalf("image model: %v", err)
	}
	_, err = model.DoGenerate(context.Background(), provider.ImageModelV3CallOptions{Prompt: "A neon skyline"})
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
}

func TestProdiaImageModelAPIError(t *testing.T) {
	t.Setenv("PRODIA_TOKEN", "test-key")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprint(w, `{"message":"Invalid prompt","detail":"Prompt cannot be empty"}`)
	}))
	defer server.Close()

	client := CreateProdia(Settings{BaseURL: server.URL})
	model, err := client.ImageModel("inference.flux-fast.schnell.txt2img.v2")
	if err != nil {
		t.Fatalf("image model: %v", err)
	}
	_, err = model.DoGenerate(context.Background(), provider.ImageModelV3CallOptions{Prompt: ""})
	if err == nil {
		t.Fatalf("expected error")
	}
	var invalidErr *provider.InvalidRequestError
	if !errors.As(err, &invalidErr) {
		t.Fatalf("expected invalid request error, got %T", err)
	}
	if invalidErr.Message != "Prompt cannot be empty" {
		t.Fatalf("unexpected error message: %s", invalidErr.Message)
	}
}

func createProdiaMultipartResponse(t *testing.T, jobResult map[string]any, image []byte) ([]byte, string) {
	t.Helper()
	buffer := &bytes.Buffer{}
	writer := multipart.NewWriter(buffer)

	jobHeader := make(textproto.MIMEHeader)
	jobHeader.Set("Content-Disposition", "form-data; name=\"job\"; filename=\"job.json\"")
	jobHeader.Set("Content-Type", "application/json")
	jobPart, err := writer.CreatePart(jobHeader)
	if err != nil {
		t.Fatalf("create job part: %v", err)
	}
	payload, err := json.Marshal(jobResult)
	if err != nil {
		t.Fatalf("marshal job: %v", err)
	}
	if _, err := jobPart.Write(payload); err != nil {
		t.Fatalf("write job: %v", err)
	}

	outputHeader := make(textproto.MIMEHeader)
	outputHeader.Set("Content-Disposition", "form-data; name=\"output\"; filename=\"output.png\"")
	outputHeader.Set("Content-Type", "image/png")
	outputPart, err := writer.CreatePart(outputHeader)
	if err != nil {
		t.Fatalf("create output part: %v", err)
	}
	if _, err := outputPart.Write(image); err != nil {
		t.Fatalf("write output: %v", err)
	}

	if err := writer.Close(); err != nil {
		t.Fatalf("close multipart: %v", err)
	}

	return buffer.Bytes(), writer.FormDataContentType()
}
