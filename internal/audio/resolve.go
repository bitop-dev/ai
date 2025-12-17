package audio

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"time"
)

func ResolveInput(ctx context.Context, audioBytes []byte, audioBase64 string, audioURL string, mediaType string, filename string) ([]byte, string, string, error) {
	if len(audioBytes) > 0 {
		return audioBytes, defaultString(mediaType, "application/octet-stream"), defaultString(filename, "audio"), nil
	}
	if audioBase64 != "" {
		b, err := base64.StdEncoding.DecodeString(audioBase64)
		if err != nil {
			return nil, "", "", fmt.Errorf("audioBase64 decode: %w", err)
		}
		return b, defaultString(mediaType, "application/octet-stream"), defaultString(filename, "audio"), nil
	}
	if audioURL != "" {
		r, err := http.NewRequestWithContext(ctx, http.MethodGet, audioURL, nil)
		if err != nil {
			return nil, "", "", err
		}
		client := &http.Client{Timeout: 60 * time.Second}
		resp, err := client.Do(r)
		if err != nil {
			return nil, "", "", err
		}
		defer resp.Body.Close()
		if resp.StatusCode < 200 || resp.StatusCode > 299 {
			return nil, "", "", fmt.Errorf("audioURL http status %d", resp.StatusCode)
		}
		b, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, "", "", err
		}
		mt := mediaType
		if mt == "" {
			mt = resp.Header.Get("Content-Type")
		}
		return b, defaultString(mt, "application/octet-stream"), defaultString(filename, "audio"), nil
	}
	return nil, "", "", fmt.Errorf("audio is required (AudioBytes, AudioBase64, or AudioURL)")
}

func defaultString(v, fallback string) string {
	if v != "" {
		return v
	}
	return fallback
}
