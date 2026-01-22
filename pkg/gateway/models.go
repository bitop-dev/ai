package gateway

import (
	"context"
	"net/http"
	"regexp"

	"github.com/vercel/ai-sdk-go/pkg/provider"
)

var gatewayAnyURLPattern = regexp.MustCompile(".*")

var gatewaySupportedURLs = provider.SupportedURLPatterns{
	"*/*": []*regexp.Regexp{gatewayAnyURLPattern},
}

type GatewayLanguageModel struct {
	providerID provider.ProviderID
	modelID    provider.ModelID
	baseURL    string
	headers    map[string]string
	httpClient *http.Client
	Settings   GatewayLanguageModelSettings
}

func newGatewayLanguageModel(provider *GatewayProvider, modelID provider.ModelID) *GatewayLanguageModel {
	return &GatewayLanguageModel{
		providerID: provider.providerID,
		modelID:    modelID,
		baseURL:    provider.baseURL,
		headers:    provider.headers,
		httpClient: provider.httpClient,
		Settings: GatewayLanguageModelSettings{
			ID: GatewayModelID(modelID),
		},
	}
}

func (m *GatewayLanguageModel) SpecificationVersion() provider.SpecificationVersion {
	return provider.SpecificationVersionV3
}

func (m *GatewayLanguageModel) ProviderID() provider.ProviderID {
	return m.providerID
}

func (m *GatewayLanguageModel) ModelID() provider.ModelID {
	return m.modelID
}

func (m *GatewayLanguageModel) SupportedURLs() provider.SupportedURLPatterns {
	return gatewaySupportedURLs
}

func (m *GatewayLanguageModel) DoGenerate(ctx context.Context, options provider.LanguageModelV3CallOptions) (provider.LanguageModelV3GenerateResult, error) {
	_ = ctx
	_ = options
	return provider.LanguageModelV3GenerateResult{}, provider.NewUnsupportedFunctionalityError(
		"gateway language model generation is not implemented",
		nil,
		"language-model-generate",
	)
}

func (m *GatewayLanguageModel) DoStream(ctx context.Context, options provider.LanguageModelV3CallOptions) (provider.LanguageModelV3StreamResult, error) {
	return m.streamLanguageModel(ctx, options)
}

type GatewayEmbeddingModel struct {
	providerID provider.ProviderID
	modelID    provider.ModelID
	baseURL    string
	headers    map[string]string
	httpClient *http.Client
	Settings   GatewayEmbeddingModelSettings
}

func newGatewayEmbeddingModel(provider *GatewayProvider, modelID provider.ModelID) *GatewayEmbeddingModel {
	return &GatewayEmbeddingModel{
		providerID: provider.providerID,
		modelID:    modelID,
		baseURL:    provider.baseURL,
		headers:    provider.headers,
		httpClient: provider.httpClient,
		Settings: GatewayEmbeddingModelSettings{
			ID: GatewayEmbeddingModelID(modelID),
		},
	}
}

func (m *GatewayEmbeddingModel) SpecificationVersion() provider.SpecificationVersion {
	return provider.SpecificationVersionV3
}

func (m *GatewayEmbeddingModel) ProviderID() provider.ProviderID {
	return m.providerID
}

func (m *GatewayEmbeddingModel) ModelID() provider.ModelID {
	return m.modelID
}

func (m *GatewayEmbeddingModel) DoEmbed(ctx context.Context, options provider.EmbeddingModelV3CallOptions) (provider.EmbeddingModelV3Result, error) {
	_ = ctx
	_ = options
	return provider.EmbeddingModelV3Result{}, provider.NewUnsupportedFunctionalityError(
		"gateway embedding model calls are not implemented",
		nil,
		"embedding-model",
	)
}

type GatewayImageModel struct {
	providerID provider.ProviderID
	modelID    provider.ModelID
	baseURL    string
	headers    map[string]string
	httpClient *http.Client
	Settings   GatewayImageModelSettings
}

func newGatewayImageModel(provider *GatewayProvider, modelID provider.ModelID) *GatewayImageModel {
	return &GatewayImageModel{
		providerID: provider.providerID,
		modelID:    modelID,
		baseURL:    provider.baseURL,
		headers:    provider.headers,
		httpClient: provider.httpClient,
		Settings: GatewayImageModelSettings{
			ID: GatewayImageModelID(modelID),
		},
	}
}

func (m *GatewayImageModel) SpecificationVersion() provider.SpecificationVersion {
	return provider.SpecificationVersionV3
}

func (m *GatewayImageModel) ProviderID() provider.ProviderID {
	return m.providerID
}

func (m *GatewayImageModel) ModelID() provider.ModelID {
	return m.modelID
}

func (m *GatewayImageModel) DoGenerate(ctx context.Context, options provider.ImageModelV3CallOptions) (provider.ImageModelV3Result, error) {
	_ = ctx
	_ = options
	return provider.ImageModelV3Result{}, provider.NewUnsupportedFunctionalityError(
		"gateway image model generation is not implemented",
		nil,
		"image-model",
	)
}
