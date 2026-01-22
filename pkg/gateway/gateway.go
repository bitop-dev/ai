package gateway

import (
	"net/http"
	"os"
	"strings"

	"github.com/vercel/ai-sdk-go/pkg/provider"
)

const (
	// DefaultGatewayBaseURL is the default base URL for the gateway API.
	DefaultGatewayBaseURL = "https://gateway.ai.vercel.com"

	// GatewayAPIKeyEnvVar is the environment variable for the gateway API key.
	GatewayAPIKeyEnvVar = "AI_GATEWAY_API_KEY"
)

// GatewaySettings configures the gateway provider factory.
type GatewaySettings struct {
	APIKey     string
	BaseURL    string
	Headers    map[string]string
	HTTPClient *http.Client
	ProviderID provider.ProviderID
}

// GatewayProvider implements provider.ProviderV3 for the gateway.
type GatewayProvider struct {
	providerID provider.ProviderID
	baseURL    string
	apiKey     string
	headers    map[string]string
	httpClient *http.Client
}

// CreateGateway returns a gateway provider configured with defaults.
func CreateGateway(settings ...GatewaySettings) *GatewayProvider {
	var opts GatewaySettings
	if len(settings) > 0 {
		opts = settings[0]
	}

	baseURL := strings.TrimRight(opts.BaseURL, "/")
	if baseURL == "" {
		baseURL = DefaultGatewayBaseURL
	}

	apiKey := opts.APIKey
	if apiKey == "" {
		apiKey = os.Getenv(GatewayAPIKeyEnvVar)
	}

	providerID := opts.ProviderID
	if providerID == "" {
		providerID = provider.ProviderID("gateway")
	}

	headers := map[string]string{}
	if apiKey != "" {
		headers["Authorization"] = "Bearer " + apiKey
	}
	for key, value := range opts.Headers {
		headers[key] = value
	}

	httpClient := opts.HTTPClient
	if httpClient == nil {
		httpClient = http.DefaultClient
	}

	return &GatewayProvider{
		providerID: providerID,
		baseURL:    baseURL,
		apiKey:     apiKey,
		headers:    headers,
		httpClient: httpClient,
	}
}

// SpecificationVersion returns the provider interface version.
func (p *GatewayProvider) SpecificationVersion() provider.SpecificationVersion {
	return provider.SpecificationVersionV3
}

// LanguageModel returns a gateway language model.
func (p *GatewayProvider) LanguageModel(modelID provider.ModelID) (provider.LanguageModelV3, error) {
	return nil, provider.NewNoSuchModelError("gateway language models are not implemented", nil, p.providerID, modelID)
}

// EmbeddingModel returns a gateway embedding model.
func (p *GatewayProvider) EmbeddingModel(modelID provider.ModelID) (provider.EmbeddingModelV3, error) {
	return nil, provider.NewNoSuchModelError("gateway embedding models are not implemented", nil, p.providerID, modelID)
}

// ImageModel returns a gateway image model.
func (p *GatewayProvider) ImageModel(modelID provider.ModelID) (provider.ImageModelV3, error) {
	return nil, provider.NewNoSuchModelError("gateway image models are not implemented", nil, p.providerID, modelID)
}
