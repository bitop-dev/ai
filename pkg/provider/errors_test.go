package provider

import (
	"errors"
	"testing"
)

func TestAuthenticationErrorClassification(t *testing.T) {
	cause := errors.New("root cause")
	err := NewAuthenticationError("auth failed", cause)

	if !IsAPICallError(err) {
		t.Fatalf("expected api call classification")
	}
	if !IsAuthenticationError(err) {
		t.Fatalf("expected authentication classification")
	}
	if IsSDKError(err) {
		t.Fatalf("expected sdk classification to be false")
	}
	if !errors.Is(err, ErrAPICall) {
		t.Fatalf("expected errors.Is match for api call")
	}
	if !errors.Is(err, ErrAuthentication) {
		t.Fatalf("expected errors.Is match for authentication")
	}
	if !errors.Is(err, cause) {
		t.Fatalf("expected unwrap to expose cause")
	}
	if err.Error() != "auth failed" {
		t.Fatalf("unexpected message: %s", err.Error())
	}
}

func TestNoSuchModelErrorMetadata(t *testing.T) {
	err := NewNoSuchModelError("missing model", nil, ProviderID("openai"), ModelID("gpt-4"))

	if !IsNoSuchModelError(err) {
		t.Fatalf("expected no-such-model classification")
	}
	if !IsSDKError(err) {
		t.Fatalf("expected sdk classification")
	}
	if IsAPICallError(err) {
		t.Fatalf("expected api call classification to be false")
	}
	if err.ProviderID != ProviderID("openai") {
		t.Fatalf("unexpected provider id: %s", err.ProviderID)
	}
	if err.ModelID != ModelID("gpt-4") {
		t.Fatalf("unexpected model id: %s", err.ModelID)
	}
}

func TestInvalidPromptErrorFallbackMessage(t *testing.T) {
	err := NewInvalidPromptError("", nil)

	if err.Error() != string(ErrorKindInvalidPrompt) {
		t.Fatalf("unexpected fallback message: %s", err.Error())
	}
}
