package gateway

import (
	"errors"
	"strings"
	"testing"

	"github.com/bitop-dev/ai/pkg/provider"
)

func TestCreateGatewayErrorFromResponseInvalidFormat(t *testing.T) {
	err := CreateGatewayErrorFromResponse(GatewayErrorResponseOptions{
		Response:       "not-json",
		StatusCode:     500,
		DefaultMessage: "Gateway request failed",
	})

	var responseErr *GatewayResponseError
	if !errors.As(err, &responseErr) {
		t.Fatalf("expected GatewayResponseError, got %T", err)
	}
	if responseErr.StatusCode != 500 {
		t.Fatalf("expected status code 500, got %d", responseErr.StatusCode)
	}
	if responseErr.ValidationError == nil {
		t.Fatalf("expected validation error to be set")
	}
	if responseErr.Response != "not-json" {
		t.Fatalf("expected response to be preserved")
	}
	if !strings.Contains(responseErr.Message, "Invalid error response format") {
		t.Fatalf("unexpected message: %s", responseErr.Message)
	}
}

func TestCreateGatewayErrorFromResponseAuthentication(t *testing.T) {
	response := map[string]any{
		"error": map[string]any{
			"message": "Auth failed",
			"type":    "authentication_error",
		},
	}

	err := CreateGatewayErrorFromResponse(GatewayErrorResponseOptions{
		Response:   response,
		StatusCode: 401,
		AuthMethod: GatewayAuthMethodAPIKey,
	})

	var authErr *GatewayAuthenticationError
	if !errors.As(err, &authErr) {
		t.Fatalf("expected GatewayAuthenticationError, got %T", err)
	}
	if authErr.StatusCode != 401 {
		t.Fatalf("expected status code 401, got %d", authErr.StatusCode)
	}
	if !strings.Contains(authErr.Message, "Invalid API key") {
		t.Fatalf("expected contextual message, got %s", authErr.Message)
	}
}

func TestCreateGatewayErrorFromResponseModelNotFound(t *testing.T) {
	response := map[string]any{
		"error": map[string]any{
			"message": "Missing model",
			"type":    "model_not_found",
			"param": map[string]any{
				"modelId": "gpt-4",
			},
		},
	}

	err := CreateGatewayErrorFromResponse(GatewayErrorResponseOptions{
		Response:   response,
		StatusCode: 404,
	})

	var modelErr *GatewayModelNotFoundError
	if !errors.As(err, &modelErr) {
		t.Fatalf("expected GatewayModelNotFoundError, got %T", err)
	}
	if modelErr.ModelID != "gpt-4" {
		t.Fatalf("expected model id to be parsed, got %q", modelErr.ModelID)
	}
}

func TestMapGatewayErrorToProviderPreservesMetadata(t *testing.T) {
	gatewayErr := &GatewayInvalidRequestError{GatewayError: GatewayError{
		Type:       GatewayErrorTypeInvalidRequest,
		Message:    "Invalid request",
		StatusCode: 400,
	}}

	mapped := MapGatewayErrorToProvider(gatewayErr, GatewayErrorMetadata{
		RequestID:       "req-123",
		ResponseHeaders: map[string][]string{"X-Test": {"value"}},
		ResponseBody:    "{\"error\":true}",
		ProviderID:      provider.ProviderID("gateway"),
		ModelID:         provider.ModelID("model-1"),
	})

	var providerErr *provider.InvalidRequestError
	if !errors.As(mapped, &providerErr) {
		t.Fatalf("expected InvalidRequestError, got %T", mapped)
	}
	if providerErr.StatusCode != 400 {
		t.Fatalf("expected status code 400, got %d", providerErr.StatusCode)
	}
	if providerErr.RequestID != "req-123" {
		t.Fatalf("expected request id to be preserved")
	}
	if providerErr.ResponseBody != "{\"error\":true}" {
		t.Fatalf("expected response body to be preserved")
	}
	if providerErr.ProviderID != provider.ProviderID("gateway") {
		t.Fatalf("expected provider id to be preserved")
	}
	if providerErr.ModelID != provider.ModelID("model-1") {
		t.Fatalf("expected model id to be preserved")
	}
	if value := providerErr.ResponseHeaders["X-Test"]; len(value) != 1 || value[0] != "value" {
		t.Fatalf("expected response headers to be preserved")
	}
}
