package testserver

import (
	"bytes"
	"io"
	"net/http"
	"testing"
)

func TestServerCapturesRequestAndResponse(t *testing.T) {
	server := New(t, func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read request: %v", err)
		}
		if string(body) != "payload" {
			t.Fatalf("unexpected handler body: %s", string(body))
		}
		w.Header().Set("X-Response", "ok")
		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write([]byte("response"))
	})
	defer server.Close()

	request, err := http.NewRequest(http.MethodPost, server.URL+"/capture", bytes.NewBufferString("payload"))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	request.Header.Set("X-Request", "yes")

	response, err := server.Client().Do(request)
	if err != nil {
		t.Fatalf("do request: %v", err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatalf("read response: %v", err)
	}
	if string(body) != "response" {
		t.Fatalf("unexpected response body: %s", string(body))
	}

	capturedRequest, ok := server.LastRequest()
	if !ok {
		t.Fatalf("expected captured request")
	}
	if capturedRequest.Path != "/capture" {
		t.Fatalf("unexpected captured path: %s", capturedRequest.Path)
	}
	if capturedRequest.Header.Get("X-Request") != "yes" {
		t.Fatalf("missing captured header")
	}
	if string(capturedRequest.Body) != "payload" {
		t.Fatalf("unexpected captured body: %s", string(capturedRequest.Body))
	}

	capturedResponse, ok := server.LastResponse()
	if !ok {
		t.Fatalf("expected captured response")
	}
	if capturedResponse.Status != http.StatusAccepted {
		t.Fatalf("unexpected captured status: %d", capturedResponse.Status)
	}
	if capturedResponse.Header.Get("X-Response") != "ok" {
		t.Fatalf("missing captured response header")
	}
	if string(capturedResponse.Body) != "response" {
		t.Fatalf("unexpected captured response body: %s", string(capturedResponse.Body))
	}
}
