package provider

import (
	"context"
	"regexp"
)

// SpecificationVersion identifies the interface version implemented by a model or provider.
type SpecificationVersion string

const (
	// SpecificationVersionV3 identifies the v3 provider interfaces.
	SpecificationVersionV3 SpecificationVersion = "v3"
)

// ProviderID is a stable identifier for a provider implementation.
type ProviderID string

// ModelID is a stable identifier for a provider-specific model.
type ModelID string

// SupportedURLPatterns lists URL path patterns keyed by media type.
type SupportedURLPatterns map[string][]*regexp.Regexp

// ProviderV3 exposes accessors for the models a provider supports.
type ProviderV3 interface {
	SpecificationVersion() SpecificationVersion
	LanguageModel(modelID ModelID) (LanguageModelV3, error)
	EmbeddingModel(modelID ModelID) (EmbeddingModelV3, error)
	ImageModel(modelID ModelID) (ImageModelV3, error)
}

// TextEmbeddingProviderV3 supports the deprecated textEmbeddingModel accessor.
type TextEmbeddingProviderV3 interface {
	ProviderV3
	TextEmbeddingModel(modelID ModelID) (EmbeddingModelV3, error)
}

// TranscriptionProviderV3 exposes transcription models when available.
type TranscriptionProviderV3 interface {
	ProviderV3
	TranscriptionModel(modelID ModelID) (TranscriptionModelV3, error)
}

// SpeechProviderV3 exposes speech models when available.
type SpeechProviderV3 interface {
	ProviderV3
	SpeechModel(modelID ModelID) (SpeechModelV3, error)
}

// RerankingProviderV3 exposes reranking models when available.
type RerankingProviderV3 interface {
	ProviderV3
	RerankingModel(modelID ModelID) (RerankingModelV3, error)
}

// LanguageModelV3 is the provider-facing interface for text generation models.
type LanguageModelV3 interface {
	SpecificationVersion() SpecificationVersion
	ProviderID() ProviderID
	ModelID() ModelID
	SupportedURLs() SupportedURLPatterns
	DoGenerate(ctx context.Context, options LanguageModelV3CallOptions) (LanguageModelV3GenerateResult, error)
	DoStream(ctx context.Context, options LanguageModelV3CallOptions) (LanguageModelV3StreamResult, error)
}

// EmbeddingModelV3 is the provider-facing interface for embedding models.
type EmbeddingModelV3 interface {
	SpecificationVersion() SpecificationVersion
	ProviderID() ProviderID
	ModelID() ModelID
	DoEmbed(ctx context.Context, options EmbeddingModelV3CallOptions) (EmbeddingModelV3Result, error)
}

// ImageModelV3 is the provider-facing interface for image generation models.
type ImageModelV3 interface {
	SpecificationVersion() SpecificationVersion
	ProviderID() ProviderID
	ModelID() ModelID
	DoGenerate(ctx context.Context, options ImageModelV3CallOptions) (ImageModelV3Result, error)
}

// SpeechModelV3 is the provider-facing interface for text-to-speech models.
type SpeechModelV3 interface {
	SpecificationVersion() SpecificationVersion
	ProviderID() ProviderID
	ModelID() ModelID
	DoGenerate(ctx context.Context, options SpeechModelV3CallOptions) (SpeechModelV3Result, error)
}

// TranscriptionModelV3 is the provider-facing interface for transcription models.
type TranscriptionModelV3 interface {
	SpecificationVersion() SpecificationVersion
	ProviderID() ProviderID
	ModelID() ModelID
	DoGenerate(ctx context.Context, options TranscriptionModelV3CallOptions) (TranscriptionModelV3Result, error)
}

// RerankingModelV3 is the provider-facing interface for reranking models.
type RerankingModelV3 interface {
	SpecificationVersion() SpecificationVersion
	ProviderID() ProviderID
	ModelID() ModelID
	DoRerank(ctx context.Context, options RerankingModelV3CallOptions) (RerankingModelV3Result, error)
}

// LanguageModelV3CallOptions defines provider call options for language models.
type LanguageModelV3CallOptions struct {
	Prompt           Prompt
	MaxOutputTokens  int
	Temperature      float64
	StopSequences    []string
	TopP             float64
	TopK             int
	PresencePenalty  float64
	FrequencyPenalty float64
	ResponseFormat   *ResponseFormat
	Seed             int
	ToolChoice       *ToolChoice
	IncludeRawChunks bool
	RequestOptions   RequestOptions
	ProviderOptions  ProviderOptions
}

// LanguageModelV3GenerateResult captures a non-streaming language model response.
type LanguageModelV3GenerateResult struct{}

// LanguageModelV3StreamResult captures a streaming language model response.
type LanguageModelV3StreamResult struct {
	Stream   <-chan StreamPart
	Request  *LanguageModelV3Request
	Response *LanguageModelV3Response
}

// LanguageModelV3Request carries optional request metadata.
type LanguageModelV3Request struct {
	Body any
}

// LanguageModelV3Response carries optional response metadata.
type LanguageModelV3Response struct {
	Headers map[string][]string
}

// EmbeddingModelV3CallOptions defines provider call options for embedding models.
type EmbeddingModelV3CallOptions struct {
	Values         []string
	RequestOptions RequestOptions
}

// EmbeddingModelV3Result captures an embedding model response.
type EmbeddingModelV3Result struct{}

// ImageModelV3CallOptions defines provider call options for image models.
type ImageModelV3CallOptions struct {
	Prompt         string
	N              int
	Size           string
	AspectRatio    string
	Seed           int
	RequestOptions RequestOptions
}

// ImageModelV3Result captures an image model response.
type ImageModelV3Result struct{}

// SpeechModelV3CallOptions defines provider call options for speech models.
type SpeechModelV3CallOptions struct {
	Text           string
	Voice          string
	OutputFormat   string
	Instructions   string
	Speed          float64
	Language       string
	RequestOptions RequestOptions
}

// SpeechModelV3Result captures a speech model response.
type SpeechModelV3Result struct{}

// TranscriptionModelV3CallOptions defines provider call options for transcription models.
type TranscriptionModelV3CallOptions struct {
	Audio          []byte
	MediaType      string
	RequestOptions RequestOptions
}

// TranscriptionModelV3Result captures a transcription model response.
type TranscriptionModelV3Result struct{}

// RerankingModelV3CallOptions defines provider call options for reranking models.
type RerankingModelV3CallOptions struct{}

// RerankingModelV3Result captures a reranking model response.
type RerankingModelV3Result struct{}
