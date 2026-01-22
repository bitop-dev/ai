package fireworks

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"strings"

	"github.com/vercel/ai-sdk-go/pkg/provider"
	"github.com/vercel/ai-sdk-go/pkg/providers/openaicompatible"
)

const DefaultBaseURL = "https://api.fireworks.ai/inference/v1"
const DefaultProviderName = "fireworks"

// Settings configures the Fireworks provider.
type Settings struct {
	APIKey       string
	BaseURL      string
	Headers      map[string]string
	HTTPClient   *http.Client
	ProviderName string
}

// Provider wraps the OpenAI-compatible provider with Fireworks defaults.
type Provider struct {
	client     *openaicompatible.Provider
	providerID provider.ProviderID
	baseURL    string
}

// CreateFireworks constructs a Fireworks provider using OpenAI-compatible transport.
func CreateFireworks(settings Settings) *Provider {
	apiKey := settings.APIKey
	if apiKey == "" {
		apiKey = os.Getenv("FIREWORKS_API_KEY")
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
	model, err := p.client.LanguageModel(modelID)
	if err != nil {
		return nil, err
	}
	return &languageModel{inner: model, providerID: p.providerID}, nil
}

func (p *Provider) EmbeddingModel(modelID provider.ModelID) (provider.EmbeddingModelV3, error) {
	return p.client.EmbeddingModel(modelID)
}

func (p *Provider) ImageModel(modelID provider.ModelID) (provider.ImageModelV3, error) {
	return nil, provider.NewNoSuchModelError("fireworks does not support image models", nil, p.providerID, modelID)
}

type languageModel struct {
	inner      provider.LanguageModelV3
	providerID provider.ProviderID
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
	return m.inner.DoGenerate(ctx, options)
}

func (m *languageModel) DoStream(ctx context.Context, options provider.LanguageModelV3CallOptions) (provider.LanguageModelV3StreamResult, error) {
	includeRaw := options.IncludeRawChunks
	options.IncludeRawChunks = true
	result, err := m.inner.DoStream(ctx, options)
	if err != nil {
		return result, err
	}

	stream := make(chan provider.StreamPart)
	state := &fireworksStreamState{includeRaw: includeRaw}
	go func() {
		defer close(stream)
		for part := range result.Stream {
			if part.Type == provider.StreamPartTypeRaw {
				state.capture(part.Raw)
				if includeRaw {
					stream <- part
				}
				continue
			}
			if part.Type == provider.StreamPartTypeFinish {
				part.ProviderMetadata = mergeProviderMetadata(part.ProviderMetadata, state.metadata, m.providerID)
			}
			stream <- part
		}
	}()

	return provider.LanguageModelV3StreamResult{
		Stream:   stream,
		Request:  result.Request,
		Response: result.Response,
	}, nil
}

type fireworksStreamState struct {
	includeRaw bool
	metadata   map[string]any
}

func (s *fireworksStreamState) capture(raw any) {
	metadata := fireworksMetadataFromRaw(raw)
	if metadata == nil {
		return
	}
	if s.metadata == nil {
		s.metadata = map[string]any{}
	}
	for key, value := range metadata {
		s.metadata[key] = value
	}
}

func fireworksMetadataFromRaw(raw any) map[string]any {
	payload := normalizeRaw(raw)
	if payload == nil {
		return nil
	}
	metadata := map[string]any{}
	if value, ok := payload["safety"]; ok {
		metadata["safety"] = value
	}
	if value, ok := payload["safety_settings"]; ok {
		metadata["safety_settings"] = value
	}
	if value, ok := payload["response_metadata"]; ok {
		metadata["response_metadata"] = value
	}
	if len(metadata) == 0 {
		return nil
	}
	return metadata
}

func normalizeRaw(raw any) map[string]any {
	switch typed := raw.(type) {
	case map[string]any:
		return typed
	case string:
		var payload map[string]any
		if err := json.Unmarshal([]byte(typed), &payload); err != nil {
			return nil
		}
		return payload
	default:
		return nil
	}
}

func mergeProviderMetadata(existing provider.ProviderMetadata, updates map[string]any, providerID provider.ProviderID) provider.ProviderMetadata {
	if len(updates) == 0 {
		return existing
	}
	merged := provider.ProviderMetadata{}
	for key, value := range existing {
		copied := make(map[string]any, len(value))
		for entryKey, entryValue := range value {
			copied[entryKey] = entryValue
		}
		merged[key] = copied
	}
	if providerID == "" {
		providerID = DefaultProviderName
	}
	providerKey := string(providerID)
	if merged[providerKey] == nil {
		merged[providerKey] = map[string]any{}
	}
	for key, value := range updates {
		merged[providerKey][key] = value
	}
	return merged
}
