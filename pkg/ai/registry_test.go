package ai

import (
	"context"
	"testing"

	"github.com/vercel/ai-sdk-go/pkg/provider"
	"github.com/vercel/ai-sdk-go/pkg/registry"
)

type stubRegistryProvider struct{}

func (providerImpl *stubRegistryProvider) SpecificationVersion() provider.SpecificationVersion {
	return provider.SpecificationVersionV3
}

func (providerImpl *stubRegistryProvider) LanguageModel(provider.ModelID) (provider.LanguageModelV3, error) {
	return &stubRegistryLanguageModel{}, nil
}

func (providerImpl *stubRegistryProvider) EmbeddingModel(provider.ModelID) (provider.EmbeddingModelV3, error) {
	return nil, nil
}

func (providerImpl *stubRegistryProvider) ImageModel(provider.ModelID) (provider.ImageModelV3, error) {
	return nil, nil
}

type stubRegistryLanguageModel struct{}

func (model *stubRegistryLanguageModel) SpecificationVersion() provider.SpecificationVersion {
	return provider.SpecificationVersionV3
}

func (model *stubRegistryLanguageModel) ProviderID() provider.ProviderID { return "test" }
func (model *stubRegistryLanguageModel) ModelID() provider.ModelID       { return "alpha" }
func (model *stubRegistryLanguageModel) SupportedURLs() provider.SupportedURLPatterns {
	return nil
}
func (model *stubRegistryLanguageModel) DoGenerate(context.Context, provider.LanguageModelV3CallOptions) (provider.LanguageModelV3GenerateResult, error) {
	return provider.LanguageModelV3GenerateResult{}, nil
}
func (model *stubRegistryLanguageModel) DoStream(context.Context, provider.LanguageModelV3CallOptions) (provider.LanguageModelV3StreamResult, error) {
	ch := make(chan provider.StreamPart)
	close(ch)
	return provider.LanguageModelV3StreamResult{Stream: ch}, nil
}

func TestResolveModelUsesRegistry(t *testing.T) {
	modelRegistry := registry.NewRegistry(registry.Options{})
	modelRegistry.RegisterProvider("test", &stubRegistryProvider{})

	model, err := ResolveModel(modelRegistry, "test:alpha")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if model.ProviderID() != "test" {
		t.Fatalf("unexpected provider id: %s", model.ProviderID())
	}
}

func TestResolveModelNilRegistry(t *testing.T) {
	_, err := ResolveModel(nil, "test:alpha")
	if err != ErrNilRegistry {
		t.Fatalf("expected ErrNilRegistry, got %v", err)
	}
}
