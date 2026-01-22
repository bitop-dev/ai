package registry

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/vercel/ai-sdk-go/pkg/provider"
)

type stubLanguageModel struct {
	providerID provider.ProviderID
	modelID    provider.ModelID
}

func (model *stubLanguageModel) SpecificationVersion() provider.SpecificationVersion {
	return provider.SpecificationVersionV3
}

func (model *stubLanguageModel) ProviderID() provider.ProviderID { return model.providerID }
func (model *stubLanguageModel) ModelID() provider.ModelID       { return model.modelID }
func (model *stubLanguageModel) SupportedURLs() provider.SupportedURLPatterns {
	return nil
}
func (model *stubLanguageModel) DoGenerate(context.Context, provider.LanguageModelV3CallOptions) (provider.LanguageModelV3GenerateResult, error) {
	return provider.LanguageModelV3GenerateResult{}, nil
}
func (model *stubLanguageModel) DoStream(context.Context, provider.LanguageModelV3CallOptions) (provider.LanguageModelV3StreamResult, error) {
	ch := make(chan provider.StreamPart)
	close(ch)
	return provider.LanguageModelV3StreamResult{Stream: ch}, nil
}

type stubImageModel struct {
	providerID provider.ProviderID
	modelID    provider.ModelID
}

func (model *stubImageModel) SpecificationVersion() provider.SpecificationVersion {
	return provider.SpecificationVersionV3
}

func (model *stubImageModel) ProviderID() provider.ProviderID { return model.providerID }
func (model *stubImageModel) ModelID() provider.ModelID       { return model.modelID }
func (model *stubImageModel) DoGenerate(context.Context, provider.ImageModelV3CallOptions) (provider.ImageModelV3Result, error) {
	return provider.ImageModelV3Result{}, nil
}

type stubProvider struct {
	languageModels  map[provider.ModelID]provider.LanguageModelV3
	embeddingModels map[provider.ModelID]provider.EmbeddingModelV3
	imageModels     map[provider.ModelID]provider.ImageModelV3
}

func (providerImpl *stubProvider) SpecificationVersion() provider.SpecificationVersion {
	return provider.SpecificationVersionV3
}

func (providerImpl *stubProvider) LanguageModel(modelID provider.ModelID) (provider.LanguageModelV3, error) {
	return providerImpl.languageModels[modelID], nil
}

func (providerImpl *stubProvider) EmbeddingModel(modelID provider.ModelID) (provider.EmbeddingModelV3, error) {
	return providerImpl.embeddingModels[modelID], nil
}

func (providerImpl *stubProvider) ImageModel(modelID provider.ModelID) (provider.ImageModelV3, error) {
	return providerImpl.imageModels[modelID], nil
}

type wrappedLanguageModel struct {
	provider.LanguageModelV3
}

type wrappedImageModel struct {
	provider.ImageModelV3
}

func TestRegistryLanguageModelMiddleware(t *testing.T) {
	baseModel := &stubLanguageModel{providerID: "test", modelID: "alpha"}
	registry := NewRegistry(Options{
		LanguageModelMiddleware: []LanguageModelMiddleware{
			func(model provider.LanguageModelV3) provider.LanguageModelV3 {
				return &wrappedLanguageModel{LanguageModelV3: model}
			},
		},
	})
	registry.RegisterProvider("test", &stubProvider{
		languageModels: map[provider.ModelID]provider.LanguageModelV3{"alpha": baseModel},
	})

	model, err := registry.LanguageModel("test:alpha")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := model.(*wrappedLanguageModel); !ok {
		t.Fatalf("expected language model middleware to wrap model, got %T", model)
	}
}

func TestRegistryImageModelMiddleware(t *testing.T) {
	baseModel := &stubImageModel{providerID: "test", modelID: "img"}
	registry := NewRegistry(Options{
		ImageModelMiddleware: []ImageModelMiddleware{
			func(model provider.ImageModelV3) provider.ImageModelV3 {
				return &wrappedImageModel{ImageModelV3: model}
			},
		},
	})
	registry.RegisterProvider("test", &stubProvider{
		imageModels: map[provider.ModelID]provider.ImageModelV3{"img": baseModel},
	})

	model, err := registry.ImageModel("test:img")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := model.(*wrappedImageModel); !ok {
		t.Fatalf("expected image model middleware to wrap model, got %T", model)
	}
}

func TestRegistryMissingProvider(t *testing.T) {
	registry := NewRegistry(Options{})
	_, err := registry.LanguageModel("missing:alpha")
	if err == nil {
		t.Fatalf("expected error")
	}
	var providerErr *NoSuchProviderError
	if !errors.As(err, &providerErr) {
		t.Fatalf("expected NoSuchProviderError, got %T", err)
	}
	if providerErr.ProviderID != "missing" {
		t.Fatalf("unexpected provider id: %s", providerErr.ProviderID)
	}
}

func TestRegistryInvalidID(t *testing.T) {
	registry := NewRegistry(Options{})
	registry.RegisterProvider("test", &stubProvider{})
	_, err := registry.LanguageModel("invalid")
	if err == nil {
		t.Fatalf("expected error")
	}
	if !provider.IsNoSuchModelError(err) {
		t.Fatalf("expected no such model error, got %T", err)
	}
	if !strings.Contains(err.Error(), "invalid languageModel id") {
		t.Fatalf("expected invalid id message, got %q", err.Error())
	}
}

func TestRegistryMissingModel(t *testing.T) {
	registry := NewRegistry(Options{})
	registry.RegisterProvider("test", &stubProvider{
		languageModels: map[provider.ModelID]provider.LanguageModelV3{},
	})
	_, err := registry.LanguageModel("test:missing")
	if err == nil {
		t.Fatalf("expected error")
	}
	if !provider.IsNoSuchModelError(err) {
		t.Fatalf("expected no such model error, got %T", err)
	}
	var modelErr *provider.NoSuchModelError
	if !errors.As(err, &modelErr) {
		t.Fatalf("expected NoSuchModelError, got %T", err)
	}
	if modelErr.ProviderID != "test" {
		t.Fatalf("unexpected provider id: %s", modelErr.ProviderID)
	}
	if modelErr.ModelID != "missing" {
		t.Fatalf("unexpected model id: %s", modelErr.ModelID)
	}
}

func TestRegistrySeparator(t *testing.T) {
	model := &stubLanguageModel{providerID: "test", modelID: "alpha"}
	registry := NewRegistry(Options{Separator: "::"})
	registry.RegisterProvider("test", &stubProvider{
		languageModels: map[provider.ModelID]provider.LanguageModelV3{"alpha": model},
	})
	_, err := registry.LanguageModel("test::alpha")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRegistryUnsupportedTranscription(t *testing.T) {
	registry := NewRegistry(Options{})
	registry.RegisterProvider("test", &stubProvider{})
	_, err := registry.TranscriptionModel("test:alpha")
	if err == nil {
		t.Fatalf("expected error")
	}
	if !provider.IsUnsupportedFunctionalityError(err) {
		t.Fatalf("expected unsupported functionality error, got %T", err)
	}
}
