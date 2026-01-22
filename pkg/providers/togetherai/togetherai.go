package togetherai

import (
	"net/http"
	"os"
	"strings"

	"github.com/vercel/ai-sdk-go/pkg/provider"
	"github.com/vercel/ai-sdk-go/pkg/providers/openaicompatible"
)

const DefaultBaseURL = "https://api.together.xyz/v1"
const DefaultProviderName = "togetherai"
const DefaultModelPrefix = "togetherai/"

// Settings configures the TogetherAI provider.
type Settings struct {
	APIKey       string
	BaseURL      string
	Headers      map[string]string
	HTTPClient   *http.Client
	ProviderName string
	ModelPrefix  string
}

// Provider wraps the OpenAI-compatible provider with TogetherAI defaults.
type Provider struct {
	client      *openaicompatible.Provider
	providerID  provider.ProviderID
	baseURL     string
	modelPrefix string
}

// CreateTogetherAI constructs a TogetherAI provider using OpenAI-compatible transport.
func CreateTogetherAI(settings Settings) *Provider {
	apiKey := settings.APIKey
	if apiKey == "" {
		apiKey = os.Getenv("TOGETHER_AI_API_KEY")
	}
	baseURL := strings.TrimRight(settings.BaseURL, "/")
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}
	providerName := settings.ProviderName
	if providerName == "" {
		providerName = DefaultProviderName
	}
	modelPrefix := settings.ModelPrefix
	if modelPrefix == "" {
		modelPrefix = DefaultModelPrefix
	}
	client := openaicompatible.CreateOpenAICompatible(openaicompatible.Settings{
		APIKey:       apiKey,
		BaseURL:      baseURL,
		Headers:      settings.Headers,
		HTTPClient:   settings.HTTPClient,
		ProviderName: providerName,
	})
	return &Provider{
		client:      client,
		providerID:  provider.ProviderID(providerName),
		baseURL:     baseURL,
		modelPrefix: modelPrefix,
	}
}

func (p *Provider) SpecificationVersion() provider.SpecificationVersion {
	return provider.SpecificationVersionV3
}

func (p *Provider) LanguageModel(modelID provider.ModelID) (provider.LanguageModelV3, error) {
	return p.client.LanguageModel(p.mapModelID(modelID))
}

func (p *Provider) EmbeddingModel(modelID provider.ModelID) (provider.EmbeddingModelV3, error) {
	return p.client.EmbeddingModel(p.mapModelID(modelID))
}

func (p *Provider) ImageModel(modelID provider.ModelID) (provider.ImageModelV3, error) {
	return p.client.ImageModel(p.mapModelID(modelID))
}

func (p *Provider) mapModelID(modelID provider.ModelID) provider.ModelID {
	if p.modelPrefix == "" {
		return modelID
	}
	value := string(modelID)
	if strings.HasPrefix(value, p.modelPrefix) {
		value = strings.TrimPrefix(value, p.modelPrefix)
	}
	return provider.ModelID(value)
}
