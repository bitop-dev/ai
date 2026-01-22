package provider

// Unversioned aliases for the current provider specification.
type Provider = ProviderV3
type TextEmbeddingProvider = TextEmbeddingProviderV3
type TranscriptionProvider = TranscriptionProviderV3
type SpeechProvider = SpeechProviderV3
type RerankingProvider = RerankingProviderV3

type LanguageModel = LanguageModelV3
type EmbeddingModel = EmbeddingModelV3
type ImageModel = ImageModelV3
type SpeechModel = SpeechModelV3
type TranscriptionModel = TranscriptionModelV3
type RerankingModel = RerankingModelV3

type LanguageModelCallOptions = LanguageModelV3CallOptions
type LanguageModelGenerateResult = LanguageModelV3GenerateResult
type LanguageModelStreamResult = LanguageModelV3StreamResult
type LanguageModelRequest = LanguageModelV3Request
type LanguageModelResponse = LanguageModelV3Response

type EmbeddingModelCallOptions = EmbeddingModelV3CallOptions
type EmbeddingModelResult = EmbeddingModelV3Result

type ImageModelCallOptions = ImageModelV3CallOptions
type ImageModelResult = ImageModelV3Result

type SpeechModelCallOptions = SpeechModelV3CallOptions
type SpeechModelResult = SpeechModelV3Result

type TranscriptionModelCallOptions = TranscriptionModelV3CallOptions
type TranscriptionModelResult = TranscriptionModelV3Result

type RerankingModelCallOptions = RerankingModelV3CallOptions
type RerankingModelResult = RerankingModelV3Result

const SpecificationVersionCurrent = SpecificationVersionV3
