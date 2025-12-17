package httpx

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

type RetryPolicy struct {
	MaxRetries int
	MinBackoff time.Duration
	MaxBackoff time.Duration
}

func DoJSON(ctx context.Context, client *http.Client, method, url string, body []byte, headers http.Header, policy RetryPolicy) (*http.Response, error) {
	if client == nil {
		client = http.DefaultClient
	}
	if policy.MaxRetries < 0 {
		policy.MaxRetries = 0
	}
	if policy.MinBackoff <= 0 {
		policy.MinBackoff = 250 * time.Millisecond
	}
	if policy.MaxBackoff <= 0 {
		policy.MaxBackoff = 5 * time.Second
	}
	if policy.MaxBackoff < policy.MinBackoff {
		policy.MaxBackoff = policy.MinBackoff
	}

	var lastErr error
	for attempt := 0; attempt <= policy.MaxRetries; attempt++ {
		req, err := http.NewRequestWithContext(ctx, method, url, bytes.NewReader(body))
		if err != nil {
			return nil, err
		}
		req.Header = headers.Clone()
		if req.Header.Get("Content-Type") == "" {
			req.Header.Set("Content-Type", "application/json")
		}
		if req.Header.Get("Accept") == "" {
			req.Header.Set("Accept", "application/json")
		}

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

func shouldRetry(status int) bool {
	return status == http.StatusRequestTimeout ||
		status == http.StatusConflict ||
		status == http.StatusTooManyRequests ||
		(status >= 500 && status <= 599)
}

func isRetryableNetErr(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		return false
	}
	var ne net.Error
	return errors.As(err, &ne) && ne.Timeout()
}

var rng = struct {
	mu sync.Mutex
	r  *rand.Rand
}{
	r: rand.New(rand.NewSource(time.Now().UnixNano())),
}

func backoffWithJitter(attempt int, min, max time.Duration) time.Duration {
	backoff := min
	for i := 0; i < attempt; i++ {
		backoff *= 2
		if backoff >= max {
			backoff = max
			break
		}
	}

	rng.mu.Lock()
	n := rng.r.Int63n(int64(backoff) + 1)
	rng.mu.Unlock()

	return time.Duration(n)
}

func retryAfter(v string) (time.Duration, bool) {
	v = strings.TrimSpace(v)
	if v == "" {
		return 0, false
	}
	if n, err := strconv.Atoi(v); err == nil && n >= 0 {
		return time.Duration(n) * time.Second, true
	}
	if t, err := http.ParseTime(v); err == nil {
		d := time.Until(t)
		if d < 0 {
			d = 0
		}
		return d, true
	}
	return 0, false
}
