package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// OAuthClientCredentialsProvider implements a minimal OAuth 2.0 client credentials flow
// and provides an Authorization header value (typically "Bearer <token>").
//
// This is intentionally lightweight and optional; callers can also use HeaderProvider
// or implement HTTPTransport.AuthProvider directly.
type OAuthClientCredentialsProvider struct {
	TokenURL     string
	ClientID     string
	ClientSecret string
	Scopes       []string
	ExtraForm    map[string]string
	HTTPClient   *http.Client
	Clock        func() time.Time

	mu        sync.Mutex
	tokenType string
	token     string
	expiresAt time.Time
}

func (p *OAuthClientCredentialsProvider) AuthorizationHeader(ctx context.Context) (string, error) {
	if p == nil {
		return "", fmt.Errorf("mcp oauth: provider is nil")
	}
	now := time.Now
	if p.Clock != nil {
		now = p.Clock
	}

	p.mu.Lock()
	tt, tok, exp := p.tokenType, p.token, p.expiresAt
	p.mu.Unlock()

	// Refresh slightly before expiry to avoid edge-of-expiration races.
	if tok != "" && now().Add(15*time.Second).Before(exp) {
		return formatAuthHeader(tt, tok), nil
	}

	tt, tok, exp, err := p.fetch(ctx, now())
	if err != nil {
		return "", err
	}

	p.mu.Lock()
	p.tokenType, p.token, p.expiresAt = tt, tok, exp
	p.mu.Unlock()

	return formatAuthHeader(tt, tok), nil
}

func formatAuthHeader(tokenType, token string) string {
	if token == "" {
		return ""
	}
	if tokenType == "" {
		tokenType = "Bearer"
	}
	return strings.TrimSpace(tokenType) + " " + token
}

func (p *OAuthClientCredentialsProvider) fetch(ctx context.Context, now time.Time) (tokenType, token string, expiresAt time.Time, err error) {
	if p.TokenURL == "" {
		return "", "", time.Time{}, fmt.Errorf("mcp oauth: TokenURL is required")
	}
	if p.ClientID == "" || p.ClientSecret == "" {
		return "", "", time.Time{}, fmt.Errorf("mcp oauth: ClientID and ClientSecret are required")
	}

	form := url.Values{}
	form.Set("grant_type", "client_credentials")
	form.Set("client_id", p.ClientID)
	form.Set("client_secret", p.ClientSecret)
	if len(p.Scopes) > 0 {
		form.Set("scope", strings.Join(p.Scopes, " "))
	}
	for k, v := range p.ExtraForm {
		if k != "" && v != "" {
			form.Set(k, v)
		}
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.TokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return "", "", time.Time{}, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	client := p.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}

	resp, err := client.Do(req)
	if err != nil {
		return "", "", time.Time{}, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", "", time.Time{}, err
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return "", "", time.Time{}, &HTTPStatusError{Method: http.MethodPost, URL: p.TokenURL, StatusCode: resp.StatusCode, Body: body}
	}

	var parsed struct {
		AccessToken string `json:"access_token"`
		TokenType   string `json:"token_type"`
		ExpiresIn   int64  `json:"expires_in"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return "", "", time.Time{}, err
	}
	if parsed.AccessToken == "" {
		return "", "", time.Time{}, fmt.Errorf("mcp oauth: empty access_token")
	}

	exp := now.Add(1 * time.Hour)
	if parsed.ExpiresIn > 0 {
		exp = now.Add(time.Duration(parsed.ExpiresIn) * time.Second)
	}
	return parsed.TokenType, parsed.AccessToken, exp, nil
}
