package gateway

import (
	"net/http"
	"testing"

	"github.com/bitop-dev/ai/pkg/provider"
)

func TestCreateGatewayDefaults(t *testing.T) {
	t.Setenv(GatewayAPIKeyEnvVar, "env-token")

	gatewayProvider := CreateGateway()
	if gatewayProvider.baseURL != DefaultGatewayBaseURL {
		t.Fatalf("expected base URL %q, got %q", DefaultGatewayBaseURL, gatewayProvider.baseURL)
	}
	if gatewayProvider.providerID != provider.ProviderID("gateway") {
		t.Fatalf("expected provider ID to be gateway, got %q", gatewayProvider.providerID)
	}
	if gatewayProvider.apiKey != "env-token" {
		t.Fatalf("expected api key to be loaded from env")
	}
	if gatewayProvider.headers["Authorization"] != "Bearer env-token" {
		t.Fatalf("expected authorization header to be set")
	}
	if gatewayProvider.httpClient != http.DefaultClient {
		t.Fatalf("expected http client to default to http.DefaultClient")
	}
}

func TestCreateGatewayOverrides(t *testing.T) {
	customClient := &http.Client{}
	gatewayProvider := CreateGateway(GatewaySettings{
		APIKey:     "direct-token",
		BaseURL:    "https://example.test",
		Headers:    map[string]string{"X-Test": "true", "Authorization": "Token override"},
		HTTPClient: customClient,
		ProviderID: provider.ProviderID("custom"),
	})

	if gatewayProvider.baseURL != "https://example.test" {
		t.Fatalf("expected base URL override to be applied")
	}
	if gatewayProvider.providerID != provider.ProviderID("custom") {
		t.Fatalf("expected provider ID override to be applied")
	}
	if gatewayProvider.apiKey != "direct-token" {
		t.Fatalf("expected api key override to be applied")
	}
	if gatewayProvider.headers["Authorization"] != "Token override" {
		t.Fatalf("expected custom authorization header to take precedence")
	}
	if gatewayProvider.headers["X-Test"] != "true" {
		t.Fatalf("expected custom header to be preserved")
	}
	if gatewayProvider.httpClient != customClient {
		t.Fatalf("expected custom http client to be applied")
	}
}
