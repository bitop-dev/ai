package registry

import (
	"fmt"
	"sort"
	"strings"

	"github.com/bitop-dev/ai/pkg/provider"
)

// ModelType identifies the model category for registry operations.
type ModelType string

const (
	ModelTypeLanguage      ModelType = "languageModel"
	ModelTypeEmbedding     ModelType = "embeddingModel"
	ModelTypeImage         ModelType = "imageModel"
	ModelTypeTranscription ModelType = "transcriptionModel"
	ModelTypeSpeech        ModelType = "speechModel"
	ModelTypeReranking     ModelType = "rerankingModel"
)

// LanguageModelMiddleware wraps a language model with additional behavior.
type LanguageModelMiddleware func(model provider.LanguageModelV3) provider.LanguageModelV3

// ImageModelMiddleware wraps an image model with additional behavior.
type ImageModelMiddleware func(model provider.ImageModelV3) provider.ImageModelV3

// NoSuchProviderError indicates a requested provider is missing from the registry.
type NoSuchProviderError struct {
	ProviderID         provider.ProviderID
	ModelType          ModelType
	AvailableProviders []provider.ProviderID
	Message            string
}

func (err *NoSuchProviderError) Error() string {
	if err == nil {
		return ""
	}
	if err.Message != "" {
		return err.Message
	}
	providerID := string(err.ProviderID)
	if providerID == "" {
		providerID = "<unknown>"
	}
	return fmt.Sprintf("no such provider %q for %s", providerID, err.ModelType)
}

// ProviderRegistry resolves provider models by registry identifier.
type ProviderRegistry interface {
	RegisterProvider(id provider.ProviderID, provider provider.ProviderV3)
	LanguageModel(id string) (provider.LanguageModelV3, error)
	EmbeddingModel(id string) (provider.EmbeddingModelV3, error)
	ImageModel(id string) (provider.ImageModelV3, error)
	TranscriptionModel(id string) (provider.TranscriptionModelV3, error)
	SpeechModel(id string) (provider.SpeechModelV3, error)
	RerankingModel(id string) (provider.RerankingModelV3, error)
}

// Options configures the provider registry behavior.
type Options struct {
	Separator               string
	LanguageModelMiddleware []LanguageModelMiddleware
	ImageModelMiddleware    []ImageModelMiddleware
}

// Registry provides access to registered providers by model identifier.
type Registry struct {
	providers               map[provider.ProviderID]provider.ProviderV3
	separator               string
	languageModelMiddleware []LanguageModelMiddleware
	imageModelMiddleware    []ImageModelMiddleware
}

// NewRegistry constructs a registry with the provided options.
func NewRegistry(options Options) *Registry {
	separator := options.Separator
	if separator == "" {
		separator = ":"
	}
	return &Registry{
		providers:               map[provider.ProviderID]provider.ProviderV3{},
		separator:               separator,
		languageModelMiddleware: append([]LanguageModelMiddleware(nil), options.LanguageModelMiddleware...),
		imageModelMiddleware:    append([]ImageModelMiddleware(nil), options.ImageModelMiddleware...),
	}
}

// CreateProviderRegistry builds and seeds a registry with providers.
func CreateProviderRegistry(providers map[provider.ProviderID]provider.ProviderV3, options Options) *Registry {
	registry := NewRegistry(options)
	for id, providerImpl := range providers {
		registry.RegisterProvider(id, providerImpl)
	}
	return registry
}

// RegisterProvider adds a provider to the registry.
func (registry *Registry) RegisterProvider(id provider.ProviderID, providerImpl provider.ProviderV3) {
	if registry.providers == nil {
		registry.providers = map[provider.ProviderID]provider.ProviderV3{}
	}
	registry.providers[id] = providerImpl
}

// LanguageModel resolves a language model by registry identifier.
func (registry *Registry) LanguageModel(id string) (provider.LanguageModelV3, error) {
	providerID, modelID, err := registry.splitID(id, ModelTypeLanguage)
	if err != nil {
		return nil, err
	}
	providerImpl, err := registry.getProvider(providerID, ModelTypeLanguage)
	if err != nil {
		return nil, err
	}
	model, err := providerImpl.LanguageModel(modelID)
	if err != nil {
		return nil, err
	}
	if model == nil {
		return nil, provider.NewNoSuchModelError("no such language model", nil, providerID, modelID)
	}
	for _, middleware := range registry.languageModelMiddleware {
		model = middleware(model)
	}
	return model, nil
}

// EmbeddingModel resolves an embedding model by registry identifier.
func (registry *Registry) EmbeddingModel(id string) (provider.EmbeddingModelV3, error) {
	providerID, modelID, err := registry.splitID(id, ModelTypeEmbedding)
	if err != nil {
		return nil, err
	}
	providerImpl, err := registry.getProvider(providerID, ModelTypeEmbedding)
	if err != nil {
		return nil, err
	}
	model, err := providerImpl.EmbeddingModel(modelID)
	if err != nil {
		return nil, err
	}
	if model == nil {
		return nil, provider.NewNoSuchModelError("no such embedding model", nil, providerID, modelID)
	}
	return model, nil
}

// ImageModel resolves an image model by registry identifier.
func (registry *Registry) ImageModel(id string) (provider.ImageModelV3, error) {
	providerID, modelID, err := registry.splitID(id, ModelTypeImage)
	if err != nil {
		return nil, err
	}
	providerImpl, err := registry.getProvider(providerID, ModelTypeImage)
	if err != nil {
		return nil, err
	}
	model, err := providerImpl.ImageModel(modelID)
	if err != nil {
		return nil, err
	}
	if model == nil {
		return nil, provider.NewNoSuchModelError("no such image model", nil, providerID, modelID)
	}
	for _, middleware := range registry.imageModelMiddleware {
		model = middleware(model)
	}
	return model, nil
}

// TranscriptionModel resolves a transcription model by registry identifier.
func (registry *Registry) TranscriptionModel(id string) (provider.TranscriptionModelV3, error) {
	providerID, modelID, err := registry.splitID(id, ModelTypeTranscription)
	if err != nil {
		return nil, err
	}
	providerImpl, err := registry.getProvider(providerID, ModelTypeTranscription)
	if err != nil {
		return nil, err
	}
	transcriptionProvider, ok := providerImpl.(provider.TranscriptionProviderV3)
	if !ok {
		return nil, provider.NewUnsupportedFunctionalityError("provider does not support transcription models", nil, string(ModelTypeTranscription))
	}
	model, err := transcriptionProvider.TranscriptionModel(modelID)
	if err != nil {
		return nil, err
	}
	if model == nil {
		return nil, provider.NewNoSuchModelError("no such transcription model", nil, providerID, modelID)
	}
	return model, nil
}

// SpeechModel resolves a speech model by registry identifier.
func (registry *Registry) SpeechModel(id string) (provider.SpeechModelV3, error) {
	providerID, modelID, err := registry.splitID(id, ModelTypeSpeech)
	if err != nil {
		return nil, err
	}
	providerImpl, err := registry.getProvider(providerID, ModelTypeSpeech)
	if err != nil {
		return nil, err
	}
	speechProvider, ok := providerImpl.(provider.SpeechProviderV3)
	if !ok {
		return nil, provider.NewUnsupportedFunctionalityError("provider does not support speech models", nil, string(ModelTypeSpeech))
	}
	model, err := speechProvider.SpeechModel(modelID)
	if err != nil {
		return nil, err
	}
	if model == nil {
		return nil, provider.NewNoSuchModelError("no such speech model", nil, providerID, modelID)
	}
	return model, nil
}

// RerankingModel resolves a reranking model by registry identifier.
func (registry *Registry) RerankingModel(id string) (provider.RerankingModelV3, error) {
	providerID, modelID, err := registry.splitID(id, ModelTypeReranking)
	if err != nil {
		return nil, err
	}
	providerImpl, err := registry.getProvider(providerID, ModelTypeReranking)
	if err != nil {
		return nil, err
	}
	rerankingProvider, ok := providerImpl.(provider.RerankingProviderV3)
	if !ok {
		return nil, provider.NewUnsupportedFunctionalityError("provider does not support reranking models", nil, string(ModelTypeReranking))
	}
	model, err := rerankingProvider.RerankingModel(modelID)
	if err != nil {
		return nil, err
	}
	if model == nil {
		return nil, provider.NewNoSuchModelError("no such reranking model", nil, providerID, modelID)
	}
	return model, nil
}

func (registry *Registry) splitID(id string, modelType ModelType) (provider.ProviderID, provider.ModelID, error) {
	separator := registry.separator
	index := strings.Index(id, separator)
	if index == -1 {
		return "", "", provider.NewNoSuchModelError(
			fmt.Sprintf("invalid %s id for registry: %s (must be in the format \"providerId%[2]smodelId\")", modelType, separator),
			nil,
			"",
			provider.ModelID(id),
		)
	}
	providerPart := id[:index]
	modelPart := id[index+len(separator):]
	if providerPart == "" || modelPart == "" {
		return "", "", provider.NewNoSuchModelError(
			fmt.Sprintf("invalid %s id for registry: %s (must be in the format \"providerId%[2]smodelId\")", modelType, separator),
			nil,
			"",
			provider.ModelID(id),
		)
	}
	return provider.ProviderID(providerPart), provider.ModelID(modelPart), nil
}

func (registry *Registry) getProvider(id provider.ProviderID, modelType ModelType) (provider.ProviderV3, error) {
	providerImpl, ok := registry.providers[id]
	if !ok {
		return nil, &NoSuchProviderError{
			ProviderID:         id,
			ModelType:          modelType,
			AvailableProviders: registry.availableProviders(),
			Message: fmt.Sprintf(
				"no such provider %q for %s", id, modelType,
			),
		}
	}
	return providerImpl, nil
}

func (registry *Registry) availableProviders() []provider.ProviderID {
	if len(registry.providers) == 0 {
		return nil
	}
	providers := make([]provider.ProviderID, 0, len(registry.providers))
	for id := range registry.providers {
		providers = append(providers, id)
	}
	sort.Slice(providers, func(i, j int) bool { return providers[i] < providers[j] })
	return providers
}
