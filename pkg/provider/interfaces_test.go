package provider

import (
	"context"
	"testing"
)

type stubProvider struct{}

func (stubProvider) SpecificationVersion() SpecificationVersion {
	return SpecificationVersionV3
}

func (stubProvider) LanguageModel(modelID ModelID) (LanguageModelV3, error) {
	return stubLanguageModel{modelID: modelID}, nil
}

func (stubProvider) EmbeddingModel(modelID ModelID) (EmbeddingModelV3, error) {
	return stubEmbeddingModel{modelID: modelID}, nil
}

func (stubProvider) ImageModel(modelID ModelID) (ImageModelV3, error) {
	return stubImageModel{modelID: modelID}, nil
}

func (stubProvider) TextEmbeddingModel(modelID ModelID) (EmbeddingModelV3, error) {
	return stubEmbeddingModel{modelID: modelID}, nil
}

func (stubProvider) TranscriptionModel(modelID ModelID) (TranscriptionModelV3, error) {
	return stubTranscriptionModel{modelID: modelID}, nil
}

func (stubProvider) SpeechModel(modelID ModelID) (SpeechModelV3, error) {
	return stubSpeechModel{modelID: modelID}, nil
}

func (stubProvider) RerankingModel(modelID ModelID) (RerankingModelV3, error) {
	return stubRerankingModel{modelID: modelID}, nil
}

type stubLanguageModel struct {
	modelID ModelID
}

func (stubLanguageModel) SpecificationVersion() SpecificationVersion { return SpecificationVersionV3 }
func (stubLanguageModel) ProviderID() ProviderID                     { return ProviderID("stub") }
func (m stubLanguageModel) ModelID() ModelID                         { return m.modelID }
func (stubLanguageModel) SupportedURLs() SupportedURLPatterns        { return nil }
func (stubLanguageModel) DoGenerate(ctx context.Context, options LanguageModelV3CallOptions) (LanguageModelV3GenerateResult, error) {
	return LanguageModelV3GenerateResult{}, nil
}
func (stubLanguageModel) DoStream(ctx context.Context, options LanguageModelV3CallOptions) (LanguageModelV3StreamResult, error) {
	return LanguageModelV3StreamResult{}, nil
}

type stubEmbeddingModel struct {
	modelID ModelID
}

func (stubEmbeddingModel) SpecificationVersion() SpecificationVersion { return SpecificationVersionV3 }
func (stubEmbeddingModel) ProviderID() ProviderID                     { return ProviderID("stub") }
func (m stubEmbeddingModel) ModelID() ModelID                         { return m.modelID }
func (stubEmbeddingModel) DoEmbed(ctx context.Context, options EmbeddingModelV3CallOptions) (EmbeddingModelV3Result, error) {
	return EmbeddingModelV3Result{}, nil
}

type stubImageModel struct {
	modelID ModelID
}

func (stubImageModel) SpecificationVersion() SpecificationVersion { return SpecificationVersionV3 }
func (stubImageModel) ProviderID() ProviderID                     { return ProviderID("stub") }
func (m stubImageModel) ModelID() ModelID                         { return m.modelID }
func (stubImageModel) DoGenerate(ctx context.Context, options ImageModelV3CallOptions) (ImageModelV3Result, error) {
	return ImageModelV3Result{}, nil
}

type stubSpeechModel struct {
	modelID ModelID
}

func (stubSpeechModel) SpecificationVersion() SpecificationVersion { return SpecificationVersionV3 }
func (stubSpeechModel) ProviderID() ProviderID                     { return ProviderID("stub") }
func (m stubSpeechModel) ModelID() ModelID                         { return m.modelID }
func (stubSpeechModel) DoGenerate(ctx context.Context, options SpeechModelV3CallOptions) (SpeechModelV3Result, error) {
	return SpeechModelV3Result{}, nil
}

type stubTranscriptionModel struct {
	modelID ModelID
}

func (stubTranscriptionModel) SpecificationVersion() SpecificationVersion {
	return SpecificationVersionV3
}
func (stubTranscriptionModel) ProviderID() ProviderID { return ProviderID("stub") }
func (m stubTranscriptionModel) ModelID() ModelID     { return m.modelID }
func (stubTranscriptionModel) DoGenerate(ctx context.Context, options TranscriptionModelV3CallOptions) (TranscriptionModelV3Result, error) {
	return TranscriptionModelV3Result{}, nil
}

type stubRerankingModel struct {
	modelID ModelID
}

func (stubRerankingModel) SpecificationVersion() SpecificationVersion { return SpecificationVersionV3 }
func (stubRerankingModel) ProviderID() ProviderID                     { return ProviderID("stub") }
func (m stubRerankingModel) ModelID() ModelID                         { return m.modelID }
func (stubRerankingModel) DoRerank(ctx context.Context, options RerankingModelV3CallOptions) (RerankingModelV3Result, error) {
	return RerankingModelV3Result{}, nil
}

func TestProviderInterfaces(t *testing.T) {
	provider := stubProvider{}
	if provider.SpecificationVersion() != SpecificationVersionV3 {
		t.Fatalf("expected provider spec %q", SpecificationVersionV3)
	}

	if _, err := provider.LanguageModel("language"); err != nil {
		t.Fatalf("expected language model, got %v", err)
	}

	if _, err := provider.EmbeddingModel("embedding"); err != nil {
		t.Fatalf("expected embedding model, got %v", err)
	}

	if _, err := provider.ImageModel("image"); err != nil {
		t.Fatalf("expected image model, got %v", err)
	}
}

var _ ProviderV3 = stubProvider{}
var _ TextEmbeddingProviderV3 = stubProvider{}
var _ TranscriptionProviderV3 = stubProvider{}
var _ SpeechProviderV3 = stubProvider{}
var _ RerankingProviderV3 = stubProvider{}
