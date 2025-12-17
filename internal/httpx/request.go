package httpx

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Do sends an HTTP request with a buffered body. It applies the provided retry
// policy; callers must close the returned response body.
func Do(ctx context.Context, client *http.Client, method, url string, body []byte, headers http.Header, policy RetryPolicy) (*http.Response, error) {
	if client == nil {
		client = http.DefaultClient
	}

	var lastErr error
	for attempt := 0; attempt <= policy.MaxRetries; attempt++ {
		req, err := http.NewRequestWithContext(ctx, method, url, bytes.NewReader(body))
		if err != nil {
			return nil, err
		}
		req.Header = headers.Clone()

		resp, err := client.Do(req)
		if err == nil && resp != nil && !shouldRetry(resp.StatusCode) {
			return resp, nil
		}

		if err == nil && resp != nil {
			_, _ = io.Copy(io.Discard, resp.Body)
			resp.Body.Close()
			lastErr = fmt.Errorf("http status %d", resp.StatusCode)
		} else {
			lastErr = err
		}

		if attempt == policy.MaxRetries {
			break
		}
		if err != nil && !isRetryableNetErr(err) {
			break
		}

		sleep := backoffWithJitter(attempt, policy.MinBackoff, policy.MaxBackoff)
		if resp != nil {
			if ra, ok := retryAfter(resp.Header.Get("Retry-After")); ok && ra > sleep {
				sleep = ra
			}
		}
		if sleep > 0 {
			timer := time.NewTimer(sleep)
			select {
			case <-ctx.Done():
				timer.Stop()
				return nil, ctx.Err()
			case <-timer.C:
			}
		}
	}
	return nil, lastErr
}
