package providerutils

import (
	"bytes"
	"context"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/bitop-dev/ai/pkg/provider"
)

func TestPostJSONAppliesRequestOptions(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", r.Method)
		}
		if r.Header.Get("X-Test") != "ok" {
			t.Fatalf("expected custom header to be set")
		}
		if r.Header.Get("Idempotency-Key") != "idem-123" {
			t.Fatalf("expected idempotency key to be set")
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Fatalf("expected JSON content type")
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read body: %v", err)
		}
		if string(body) != `{"foo":"bar"}` {
			t.Fatalf("unexpected body: %s", string(body))
		}
		_, _ = w.Write([]byte("ok"))
	}))
	defer server.Close()

	options := provider.RequestOptions{
		Headers:        map[string]string{"X-Test": "ok"},
		IdempotencyKey: "idem-123",
	}

	response, err := PostJSON(context.Background(), server.Client(), server.URL, map[string]string{"foo": "bar"}, options, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(response.Body) != "ok" {
		t.Fatalf("unexpected response body: %s", string(response.Body))
	}
}

func TestPostMultipartBuildsPayload(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reader, err := r.MultipartReader()
		if err != nil {
			t.Fatalf("multipart reader: %v", err)
		}
		form, err := reader.ReadForm(1024 * 1024)
		if err != nil {
			t.Fatalf("read form: %v", err)
		}
		if form.Value["name"][0] != "example" {
			t.Fatalf("unexpected form value")
		}
		files := form.File["file"]
		if len(files) != 1 {
			t.Fatalf("expected one file")
		}
		file, err := files[0].Open()
		if err != nil {
			t.Fatalf("open file: %v", err)
		}
		defer file.Close()
		content, err := io.ReadAll(file)
		if err != nil {
			t.Fatalf("read file: %v", err)
		}
		if string(content) != "payload" {
			t.Fatalf("unexpected file content: %s", string(content))
		}
		_, _ = w.Write([]byte("ok"))
	}))
	defer server.Close()

	payload := MultipartPayload{
		Fields: map[string]string{"name": "example"},
		Files: []MultipartFile{{
			FieldName:   "file",
			FileName:    "test.txt",
			ContentType: "text/plain",
			Content:     []byte("payload"),
		}},
	}

	response, err := PostMultipart(context.Background(), server.Client(), server.URL, payload, provider.RequestOptions{}, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(response.Body) != "ok" {
		t.Fatalf("unexpected response body: %s", string(response.Body))
	}
}

func TestRetryPolicyRetriesOnStatus(t *testing.T) {
	var calls int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if calls == 1 {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte("fail"))
			return
		}
		_, _ = w.Write([]byte("ok"))
	}))
	defer server.Close()

	policy := RetryPolicy{
		MaxRetries:           1,
		BaseDelay:            0,
		MaxDelay:             0,
		RetryableStatusCodes: map[int]bool{http.StatusInternalServerError: true},
		UseRetryAfterHeader:  true,
	}

	response, err := PostJSON(context.Background(), server.Client(), server.URL, map[string]string{"foo": "bar"}, provider.RequestOptions{}, &policy, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if calls != 2 {
		t.Fatalf("expected 2 calls, got %d", calls)
	}
	if string(response.Body) != "ok" {
		t.Fatalf("unexpected response body: %s", string(response.Body))
	}
}

func TestBackoffDelayHonorsMax(t *testing.T) {
	delay := BackoffDelay(time.Millisecond, 4, 5*time.Millisecond)
	if delay != 5*time.Millisecond {
		t.Fatalf("expected capped delay, got %v", delay)
	}
}

func TestBuildRequestSetsTimeoutAndHeaders(t *testing.T) {
	ctx := context.Background()
	options := provider.RequestOptions{
		Headers:        map[string]string{"X-Test": "ok"},
		IdempotencyKey: "key",
		Timeout:        time.Second,
	}
	request, cancel, err := BuildRequest(ctx, http.MethodPost, "http://example.com", bytes.NewReader([]byte("data")), map[string]string{"Content-Type": "text/plain"}, options)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer cancel()
	if request.Header.Get("X-Test") != "ok" {
		t.Fatalf("expected header to be applied")
	}
	if request.Header.Get("Idempotency-Key") != "key" {
		t.Fatalf("expected idempotency header")
	}
	if request.Header.Get("Content-Type") != "text/plain" {
		t.Fatalf("expected content type header")
	}
}

func TestMultipartBodyIncludesFileContent(t *testing.T) {
	payload := MultipartPayload{
		Fields: map[string]string{"field": "value"},
		Files: []MultipartFile{{
			FieldName: "upload",
			FileName:  "data.txt",
			Content:   []byte("data"),
		}},
	}
	body, contentType, err := buildMultipartBody(payload)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	_, params, err := mime.ParseMediaType(contentType)
	if err != nil {
		t.Fatalf("parse content type: %v", err)
	}
	boundary := params["boundary"]
	if boundary == "" {
		t.Fatalf("expected boundary to be set")
	}
	reader := multipart.NewReader(bytes.NewReader(body), boundary)
	form, err := reader.ReadForm(1024 * 1024)
	if err != nil {
		t.Fatalf("read form: %v", err)
	}
	if form.Value["field"][0] != "value" {
		t.Fatalf("unexpected field value")
	}
	file := form.File["upload"][0]
	opened, err := file.Open()
	if err != nil {
		t.Fatalf("open file: %v", err)
	}
	defer opened.Close()
	content, err := io.ReadAll(opened)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	if string(content) != "data" {
		t.Fatalf("unexpected file content: %s", string(content))
	}
}

func TestParseRetryAfterSeconds(t *testing.T) {
	headers := http.Header{}
	headers.Set("Retry-After", strconv.Itoa(2))
	result, ok := parseRetryAfter(headers)
	if !ok {
		t.Fatalf("expected retry-after to parse")
	}
	if result != 2*time.Second {
		t.Fatalf("unexpected retry-after duration: %v", result)
	}
}
