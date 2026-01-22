package providerutils

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"mime/multipart"
	"net"
	"net/http"
	"net/textproto"
	"strconv"
	"time"

	"github.com/vercel/ai-sdk-go/pkg/provider"
)

type HTTPResponse struct {
	StatusCode int
	Headers    http.Header
	Body       []byte
}

type APICallInfo struct {
	Method          string
	URL             string
	RequestHeaders  http.Header
	RequestBody     []byte
	ResponseStatus  int
	ResponseHeaders http.Header
	ResponseBody    []byte
	Err             error
	Attempt         int
	Duration        time.Duration
	RetryIn         time.Duration
}

type APICallHook func(info APICallInfo)

type RetryPolicy struct {
	MaxRetries           int
	BaseDelay            time.Duration
	MaxDelay             time.Duration
	RetryableStatusCodes map[int]bool
	RetryableErrors      func(error) bool
	UseRetryAfterHeader  bool
}

func DefaultRetryPolicy() RetryPolicy {
	return RetryPolicy{
		MaxRetries:           2,
		BaseDelay:            200 * time.Millisecond,
		MaxDelay:             2 * time.Second,
		RetryableStatusCodes: map[int]bool{429: true, 500: true, 502: true, 503: true, 504: true},
		RetryableErrors:      IsRetryableError,
		UseRetryAfterHeader:  true,
	}
}

type MultipartFile struct {
	FieldName   string
	FileName    string
	ContentType string
	Content     []byte
}

type MultipartPayload struct {
	Fields map[string]string
	Files  []MultipartFile
}

func MergeHeaders(base, overrides map[string]string) map[string]string {
	if base == nil && overrides == nil {
		return nil
	}
	merged := make(map[string]string, len(base)+len(overrides))
	for key, value := range base {
		merged[key] = value
	}
	for key, value := range overrides {
		merged[key] = value
	}
	return merged
}

func BuildRequest(ctx context.Context, method, url string, body io.Reader, headers map[string]string, options provider.RequestOptions) (*http.Request, context.CancelFunc, error) {
	requestCtx := ctx
	cancel := func() {}
	if options.Timeout > 0 {
		requestCtx, cancel = context.WithTimeout(ctx, options.Timeout)
	}

	req, err := http.NewRequestWithContext(requestCtx, method, url, body)
	if err != nil {
		cancel()
		return nil, nil, err
	}

	mergedHeaders := MergeHeaders(headers, options.Headers)
	for key, value := range mergedHeaders {
		req.Header.Set(key, value)
	}
	if options.IdempotencyKey != "" {
		req.Header.Set("Idempotency-Key", options.IdempotencyKey)
	}

	return req, cancel, nil
}

func PostJSON(ctx context.Context, client *http.Client, url string, payload any, options provider.RequestOptions, policy *RetryPolicy, hook APICallHook) (HTTPResponse, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return HTTPResponse{}, err
	}

	return doWithRetry(ctx, client, policy, hook, func(ctx context.Context) (*http.Request, context.CancelFunc, []byte, error) {
		headers := map[string]string{"Content-Type": "application/json"}
		req, cancel, err := BuildRequest(ctx, http.MethodPost, url, bytes.NewReader(body), headers, options)
		return req, cancel, body, err
	})
}

func PostMultipart(ctx context.Context, client *http.Client, url string, payload MultipartPayload, options provider.RequestOptions, policy *RetryPolicy, hook APICallHook) (HTTPResponse, error) {
	body, contentType, err := buildMultipartBody(payload)
	if err != nil {
		return HTTPResponse{}, err
	}

	return doWithRetry(ctx, client, policy, hook, func(ctx context.Context) (*http.Request, context.CancelFunc, []byte, error) {
		headers := map[string]string{"Content-Type": contentType}
		req, cancel, err := BuildRequest(ctx, http.MethodPost, url, bytes.NewReader(body), headers, options)
		return req, cancel, body, err
	})
}

func IsRetryableError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.Canceled) {
		return false
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return false
	}
	var netErr net.Error
	if errors.As(err, &netErr) {
		return netErr.Timeout() || netErr.Temporary()
	}
	return false
}

func BackoffDelay(base time.Duration, attempt int, max time.Duration) time.Duration {
	if base <= 0 {
		return 0
	}
	if attempt < 0 {
		attempt = 0
	}
	delay := base
	for i := 0; i < attempt; i++ {
		delay *= 2
		if max > 0 && delay >= max {
			return max
		}
	}
	if max > 0 && delay > max {
		return max
	}
	return delay
}

func doWithRetry(ctx context.Context, client *http.Client, policy *RetryPolicy, hook APICallHook, builder func(context.Context) (*http.Request, context.CancelFunc, []byte, error)) (HTTPResponse, error) {
	resolvedPolicy := DefaultRetryPolicy()
	if policy != nil {
		resolvedPolicy = *policy
	}
	if client == nil {
		client = http.DefaultClient
	}

	for attempt := 0; ; attempt++ {
		if ctx.Err() != nil {
			return HTTPResponse{}, ctx.Err()
		}
		req, cancel, requestBody, err := builder(ctx)
		if err != nil {
			if cancel != nil {
				cancel()
			}
			return HTTPResponse{}, err
		}

		start := time.Now()
		resp, err := client.Do(req)
		duration := time.Since(start)
		if cancel != nil {
			cancel()
		}

		response := HTTPResponse{}
		var responseHeaders http.Header
		var responseBody []byte
		statusCode := 0
		if resp != nil {
			statusCode = resp.StatusCode
			responseHeaders = resp.Header.Clone()
			body, readErr := io.ReadAll(resp.Body)
			_ = resp.Body.Close()
			responseBody = body
			if readErr != nil && err == nil {
				err = readErr
			}
		}

		response.StatusCode = statusCode
		response.Headers = responseHeaders
		response.Body = responseBody

		retry, wait := shouldRetry(resolvedPolicy, attempt, err, statusCode, responseHeaders)
		if hook != nil {
			hook(APICallInfo{
				Method:          req.Method,
				URL:             req.URL.String(),
				RequestHeaders:  req.Header.Clone(),
				RequestBody:     requestBody,
				ResponseStatus:  statusCode,
				ResponseHeaders: responseHeaders,
				ResponseBody:    responseBody,
				Err:             err,
				Attempt:         attempt + 1,
				Duration:        duration,
				RetryIn:         wait,
			})
		}

		if !retry {
			if err != nil {
				return response, err
			}
			return response, nil
		}

		if wait > 0 {
			timer := time.NewTimer(wait)
			select {
			case <-ctx.Done():
				timer.Stop()
				return response, ctx.Err()
			case <-timer.C:
			}
		}
	}
}

func shouldRetry(policy RetryPolicy, attempt int, err error, status int, headers http.Header) (bool, time.Duration) {
	if attempt >= policy.MaxRetries {
		return false, 0
	}
	if err != nil {
		if policy.RetryableErrors != nil && policy.RetryableErrors(err) {
			return true, BackoffDelay(policy.BaseDelay, attempt, policy.MaxDelay)
		}
		return false, 0
	}
	if status == 0 {
		return false, 0
	}
	if policy.RetryableStatusCodes != nil && policy.RetryableStatusCodes[status] {
		if policy.UseRetryAfterHeader {
			if retryAfter, ok := parseRetryAfter(headers); ok {
				return true, retryAfter
			}
		}
		return true, BackoffDelay(policy.BaseDelay, attempt, policy.MaxDelay)
	}
	return false, 0
}

func parseRetryAfter(headers http.Header) (time.Duration, bool) {
	if headers == nil {
		return 0, false
	}
	value := headers.Get("Retry-After")
	if value == "" {
		return 0, false
	}
	if seconds, err := strconv.Atoi(value); err == nil {
		if seconds <= 0 {
			return 0, false
		}
		return time.Duration(seconds) * time.Second, true
	}
	parsed, err := http.ParseTime(value)
	if err != nil {
		return 0, false
	}
	wait := time.Until(parsed)
	if wait <= 0 {
		return 0, false
	}
	return wait, true
}

func buildMultipartBody(payload MultipartPayload) ([]byte, string, error) {
	buffer := &bytes.Buffer{}
	writer := multipart.NewWriter(buffer)

	for key, value := range payload.Fields {
		if err := writer.WriteField(key, value); err != nil {
			_ = writer.Close()
			return nil, "", err
		}
	}

	for _, file := range payload.Files {
		header := make(textproto.MIMEHeader)
		header.Set("Content-Disposition", "form-data; name=\""+file.FieldName+"\"; filename=\""+file.FileName+"\"")
		contentType := file.ContentType
		if contentType == "" {
			contentType = "application/octet-stream"
		}
		header.Set("Content-Type", contentType)
		part, err := writer.CreatePart(header)
		if err != nil {
			_ = writer.Close()
			return nil, "", err
		}
		if _, err := part.Write(file.Content); err != nil {
			_ = writer.Close()
			return nil, "", err
		}
	}

	if err := writer.Close(); err != nil {
		return nil, "", err
	}

	return buffer.Bytes(), writer.FormDataContentType(), nil
}
