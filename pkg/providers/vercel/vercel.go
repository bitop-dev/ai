package vercel

import (
	"net/http"
	"os"
	"strings"

	"github.com/bitop-dev/ai/pkg/provider"
	"github.com/bitop-dev/ai/pkg/providers/openaicompatible"
)

const DefaultBaseURL = "https://api.v0.dev/v1"
const DefaultProviderName = "vercel"

// Settings configures the Vercel provider.
type Settings struct {
	APIKey       string
	BaseURL      string
	Headers      map[string]string
	HTTPClient   *http.Client
	ProviderName string
}

// Provider wraps the OpenAI-compatible provider with Vercel defaults.
type Provider struct {
	client     *openaicompatible.Provider
	providerID provider.ProviderID
	baseURL    string
}

// CreateVercel constructs a Vercel provider using OpenAI-compatible transport.
func CreateVercel(settings Settings) *Provider {
	apiKey := settings.APIKey
	if apiKey == "" {
		apiKey = os.Getenv("VERCEL_API_KEY")
	}
	baseURL := strings.TrimRight(settings.BaseURL, "/")
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}
	providerName := settings.ProviderName
	if providerName == "" {
		providerName = DefaultProviderName
	}
	client := openaicompatible.CreateOpenAICompatible(openaicompatible.Settings{
		APIKey:       apiKey,
		BaseURL:      baseURL,
		Headers:      settings.Headers,
		HTTPClient:   settings.HTTPClient,
		ProviderName: providerName,
	})
	return &Provider{
		client:     client,
		providerID: provider.ProviderID(providerName),
		baseURL:    baseURL,
	}
}

func (p *Provider) SpecificationVersion() provider.SpecificationVersion {
	return provider.SpecificationVersionV3
}

func (p *Provider) LanguageModel(modelID provider.ModelID) (provider.LanguageModelV3, error) {
	return p.client.LanguageModel(modelID)
}

func (p *Provider) EmbeddingModel(modelID provider.ModelID) (provider.EmbeddingModelV3, error) {
	return nil, provider.NewNoSuchModelError("vercel does not support embedding models", nil, p.providerID, modelID)
}

func (p *Provider) ImageModel(modelID provider.ModelID) (provider.ImageModelV3, error) {
	return nil, provider.NewNoSuchModelError("vercel does not support image models", nil, p.providerID, modelID)
}
