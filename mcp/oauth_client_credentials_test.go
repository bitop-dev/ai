package mcp

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestOAuthClientCredentialsProvider_CachesToken(t *testing.T) {
	var calls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"t1","token_type":"Bearer","expires_in":3600}`))
	}))
	defer srv.Close()

	now := time.Unix(0, 0)
	p := &OAuthClientCredentialsProvider{
		TokenURL:     srv.URL,
		ClientID:     "id",
		ClientSecret: "secret",
		Clock:        func() time.Time { return now },
	}

	h1, err := p.AuthorizationHeader(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	h2, err := p.AuthorizationHeader(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if h1 != "Bearer t1" || h2 != "Bearer t1" {
		t.Fatalf("headers: %q %q", h1, h2)
	}
	if calls != 1 {
		t.Fatalf("token calls=%d", calls)
	}
}
