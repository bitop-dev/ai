package testserver

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
)

type CapturedRequest struct {
	Method string
	URL    string
	Path   string
	Header http.Header
	Body   []byte
}

type CapturedResponse struct {
	Status int
	Header http.Header
	Body   []byte
}

type Server struct {
	*httptest.Server
	mu        sync.Mutex
	requests  []CapturedRequest
	responses []CapturedResponse
}

func New(t *testing.T, handler func(http.ResponseWriter, *http.Request)) *Server {
	t.Helper()
	server := &Server{}
	server.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read request body: %v", err)
		}
		_ = r.Body.Close()
		r.Body = io.NopCloser(bytes.NewReader(body))

		request := CapturedRequest{
			Method: r.Method,
			URL:    r.URL.String(),
			Path:   r.URL.Path,
			Header: r.Header.Clone(),
			Body:   cloneBytes(body),
		}

		writer := newCaptureWriter(w)
		handler(writer, r)

		status := writer.status
		if status == 0 {
			status = http.StatusOK
		}
		response := CapturedResponse{
			Status: status,
			Header: w.Header().Clone(),
			Body:   cloneBytes(writer.body.Bytes()),
		}

		server.mu.Lock()
		server.requests = append(server.requests, request)
		server.responses = append(server.responses, response)
		server.mu.Unlock()
	}))
	return server
}

func (s *Server) LastRequest() (CapturedRequest, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.requests) == 0 {
		return CapturedRequest{}, false
	}
	request := s.requests[len(s.requests)-1]
	request.Header = request.Header.Clone()
	request.Body = cloneBytes(request.Body)
	return request, true
}

func (s *Server) LastResponse() (CapturedResponse, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.responses) == 0 {
		return CapturedResponse{}, false
	}
	response := s.responses[len(s.responses)-1]
	response.Header = response.Header.Clone()
	response.Body = cloneBytes(response.Body)
	return response, true
}

func cloneBytes(value []byte) []byte {
	if len(value) == 0 {
		return nil
	}
	copyValue := make([]byte, len(value))
	copy(copyValue, value)
	return copyValue
}

type captureWriter struct {
	http.ResponseWriter
	status int
	body   bytes.Buffer
}

func newCaptureWriter(writer http.ResponseWriter) *captureWriter {
	return &captureWriter{ResponseWriter: writer}
}

func (w *captureWriter) WriteHeader(status int) {
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

func (w *captureWriter) Write(data []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	_, _ = w.body.Write(data)
	return w.ResponseWriter.Write(data)
}

func (w *captureWriter) Flush() {
	if flusher, ok := w.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}
