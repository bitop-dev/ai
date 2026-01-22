package gateway

import (
	"context"
	"errors"
	"testing"

	"github.com/vercel/ai-sdk-go/pkg/provider"
)

func TestGatewayLanguageModelWrapper(t *testing.T) {
	gatewayProvider := CreateGateway()

	model, err := gatewayProvider.LanguageModel(provider.ModelID("test-model"))
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	wrapped, ok := model.(*GatewayLanguageModel)
	if !ok {
		t.Fatalf("expected GatewayLanguageModel wrapper")
	}

	if wrapped.ProviderID() != provider.ProviderID("gateway") {
		t.Fatalf("expected provider ID to be gateway")
	}
	if wrapped.ModelID() != provider.ModelID("test-model") {
		t.Fatalf("expected model ID to be preserved")
	}
	if wrapped.Settings.ID != GatewayModelID("test-model") {
		t.Fatalf("expected settings ID to match model ID")
	}
	if len(wrapped.SupportedURLs()) == 0 {
		t.Fatalf("expected supported URLs to be defined")
	}

	_, err = wrapped.DoGenerate(context.Background(), provider.LanguageModelV3CallOptions{})
	if !errors.Is(err, provider.ErrUnsupportedFunctionality) {
		t.Fatalf("expected unsupported functionality error")
	}
}

func TestGatewayEmbeddingModelWrapper(t *testing.T) {
	gatewayProvider := CreateGateway()

	model, err := gatewayProvider.EmbeddingModel(provider.ModelID("embed-model"))
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	wrapped, ok := model.(*GatewayEmbeddingModel)
	if !ok {
		t.Fatalf("expected GatewayEmbeddingModel wrapper")
	}

	if wrapped.Settings.ID != GatewayEmbeddingModelID("embed-model") {
		t.Fatalf("expected settings ID to match model ID")
	}

	_, err = wrapped.DoEmbed(context.Background(), provider.EmbeddingModelV3CallOptions{})
	if !errors.Is(err, provider.ErrUnsupportedFunctionality) {
		t.Fatalf("expected unsupported functionality error")
	}
}

func TestGatewayImageModelWrapper(t *testing.T) {
	gatewayProvider := CreateGateway()

	model, err := gatewayProvider.ImageModel(provider.ModelID("image-model"))
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	wrapped, ok := model.(*GatewayImageModel)
	if !ok {
		t.Fatalf("expected GatewayImageModel wrapper")
	}

	if wrapped.Settings.ID != GatewayImageModelID("image-model") {
		t.Fatalf("expected settings ID to match model ID")
	}

	_, err = wrapped.DoGenerate(context.Background(), provider.ImageModelV3CallOptions{})
	if !errors.Is(err, provider.ErrUnsupportedFunctionality) {
		t.Fatalf("expected unsupported functionality error")
	}
}
