package azure

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/bitop-dev/ai/pkg/provider"
	"github.com/bitop-dev/ai/pkg/providers/openaicompatible"
)

const DefaultAPIVersion = "v1"
const DefaultProviderName = "azure"

type Settings struct {
	APIKey                 string
	ResourceName           string
	BaseURL                string
	Headers                map[string]string
	HTTPClient             *http.Client
	ProviderName           string
	APIVersion             string
	UseDeploymentBasedURLs bool
}

type Provider struct {
	apiKey                 string
	baseURL                string
	headers                map[string]string
	httpClient             *http.Client
	apiVersion             string
	providerID             provider.ProviderID
	useDeploymentBasedURLs bool
}

func CreateAzure(settings Settings) *Provider {
	apiKey := settings.APIKey
	if apiKey == "" {
		apiKey = os.Getenv("AZURE_API_KEY")
	}

	baseURL := strings.TrimRight(settings.BaseURL, "/")
	if baseURL == "" {
		resourceName := settings.ResourceName
		if resourceName == "" {
			resourceName = os.Getenv("AZURE_RESOURCE_NAME")
		}
		if resourceName != "" {
			baseURL = fmt.Sprintf("https://%s.openai.azure.com/openai", resourceName)
		}
	}

	apiVersion := settings.APIVersion
	if apiVersion == "" {
		apiVersion = DefaultAPIVersion
	}

	providerName := settings.ProviderName
	if providerName == "" {
		providerName = DefaultProviderName
	}

	return &Provider{
		apiKey:                 apiKey,
		baseURL:                baseURL,
		headers:                settings.Headers,
		httpClient:             settings.HTTPClient,
		apiVersion:             apiVersion,
		providerID:             provider.ProviderID(providerName),
		useDeploymentBasedURLs: settings.UseDeploymentBasedURLs,
	}
}

func (p *Provider) SpecificationVersion() provider.SpecificationVersion {
	return provider.SpecificationVersionV3
}

func (p *Provider) LanguageModel(modelID provider.ModelID) (provider.LanguageModelV3, error) {
	compat, err := p.openAICompatibleProvider(modelID)
	if err != nil {
		return nil, err
	}
	model, err := compat.LanguageModel(modelID)
	if err != nil {
		return nil, err
	}
	return &languageModel{inner: model}, nil
}

func (p *Provider) EmbeddingModel(modelID provider.ModelID) (provider.EmbeddingModelV3, error) {
	compat, err := p.openAICompatibleProvider(modelID)
	if err != nil {
		return nil, err
	}
	model, err := compat.EmbeddingModel(modelID)
	if err != nil {
		return nil, err
	}
	return &embeddingModel{inner: model}, nil
}

func (p *Provider) ImageModel(modelID provider.ModelID) (provider.ImageModelV3, error) {
	compat, err := p.openAICompatibleProvider(modelID)
	if err != nil {
		return nil, err
	}
	model, err := compat.ImageModel(modelID)
	if err != nil {
		return nil, err
	}
	return &imageModel{inner: model}, nil
}

func (p *Provider) openAICompatibleProvider(modelID provider.ModelID) (*openaicompatible.Provider, error) {
	baseURL, err := p.baseURLForModel(modelID)
	if err != nil {
		return nil, err
	}
	headers, err := p.requestHeaders()
	if err != nil {
		return nil, err
	}
	queryParams := map[string]string{}
	if p.apiVersion != "" {
		queryParams["api-version"] = p.apiVersion
	}
	return openaicompatible.CreateOpenAICompatible(openaicompatible.Settings{
		BaseURL:      baseURL,
		Headers:      headers,
		QueryParams:  queryParams,
		HTTPClient:   p.httpClient,
		ProviderName: string(p.providerID),
	}), nil
}

func (p *Provider) baseURLForModel(modelID provider.ModelID) (string, error) {
	if p.baseURL == "" {
		return "", fmt.Errorf("azure base url or resource name is required")
	}
	baseURL := strings.TrimRight(p.baseURL, "/")
	if p.useDeploymentBasedURLs {
		if modelID == "" {
			return "", fmt.Errorf("azure deployment id is required")
		}
		return fmt.Sprintf("%s/deployments/%s", baseURL, modelID), nil
	}
	return baseURL + "/v1", nil
}

func (p *Provider) requestHeaders() (map[string]string, error) {
	if p.apiKey == "" {
		return nil, fmt.Errorf("azure api key is required")
	}
	headers := map[string]string{
		"api-key": p.apiKey,
	}
	for key, value := range p.headers {
		headers[key] = value
	}
	return headers, nil
}

type languageModel struct {
	inner provider.LanguageModelV3
}

func (m *languageModel) SpecificationVersion() provider.SpecificationVersion {
	return m.inner.SpecificationVersion()
}

func (m *languageModel) ProviderID() provider.ProviderID {
	return m.inner.ProviderID()
}

func (m *languageModel) ModelID() provider.ModelID {
	return m.inner.ModelID()
}

func (m *languageModel) SupportedURLs() provider.SupportedURLPatterns {
	return m.inner.SupportedURLs()
}

func (m *languageModel) DoGenerate(ctx context.Context, options provider.LanguageModelV3CallOptions) (provider.LanguageModelV3GenerateResult, error) {
	result, err := m.inner.DoGenerate(ctx, options)
	if err != nil {
		return result, normalizeAzureError(err)
	}
	return result, nil
}

func (m *languageModel) DoStream(ctx context.Context, options provider.LanguageModelV3CallOptions) (provider.LanguageModelV3StreamResult, error) {
	result, err := m.inner.DoStream(ctx, options)
	if err != nil {
		return result, normalizeAzureError(err)
	}
	return result, nil
}

type embeddingModel struct {
	inner provider.EmbeddingModelV3
}

func (m *embeddingModel) SpecificationVersion() provider.SpecificationVersion {
	return m.inner.SpecificationVersion()
}

func (m *embeddingModel) ProviderID() provider.ProviderID {
	return m.inner.ProviderID()
}

func (m *embeddingModel) ModelID() provider.ModelID {
	return m.inner.ModelID()
}

func (m *embeddingModel) DoEmbed(ctx context.Context, options provider.EmbeddingModelV3CallOptions) (provider.EmbeddingModelV3Result, error) {
	result, err := m.inner.DoEmbed(ctx, options)
	if err != nil {
		return result, normalizeAzureError(err)
	}
	return result, nil
}

type imageModel struct {
	inner provider.ImageModelV3
}

func (m *imageModel) SpecificationVersion() provider.SpecificationVersion {
	return m.inner.SpecificationVersion()
}

func (m *imageModel) ProviderID() provider.ProviderID {
	return m.inner.ProviderID()
}

func (m *imageModel) ModelID() provider.ModelID {
	return m.inner.ModelID()
}

func (m *imageModel) DoGenerate(ctx context.Context, options provider.ImageModelV3CallOptions) (provider.ImageModelV3Result, error) {
	result, err := m.inner.DoGenerate(ctx, options)
	if err != nil {
		return result, normalizeAzureError(err)
	}
	return result, nil
}

func normalizeAzureError(err error) error {
	if err == nil {
		return nil
	}
	switch typed := err.(type) {
	case *provider.AuthenticationError:
		typed.RequestID = azureRequestID(typed.RequestID, typed.ResponseHeaders)
	case *provider.RateLimitError:
		typed.RequestID = azureRequestID(typed.RequestID, typed.ResponseHeaders)
	case *provider.InvalidRequestError:
		typed.RequestID = azureRequestID(typed.RequestID, typed.ResponseHeaders)
	case *provider.InternalServerError:
		typed.RequestID = azureRequestID(typed.RequestID, typed.ResponseHeaders)
	case *provider.ApiCallError:
		typed.RequestID = azureRequestID(typed.RequestID, typed.ResponseHeaders)
	}
	return err
}

func azureRequestID(existing string, headers map[string][]string) string {
	if existing != "" {
		return existing
	}
	if headers == nil {
		return ""
	}
	for key, values := range headers {
		if !isAzureRequestIDHeader(key) {
			continue
		}
		if len(values) > 0 {
			return values[0]
		}
	}
	return ""
}

func isAzureRequestIDHeader(key string) bool {
	return strings.EqualFold(key, "x-request-id") ||
		strings.EqualFold(key, "x-ms-request-id") ||
		strings.EqualFold(key, "x-ms-client-request-id")
}
