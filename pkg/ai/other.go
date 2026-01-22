package ai

import (
	"context"

	"github.com/vercel/ai-sdk-go/pkg/provider"
)

func Embed(ctx context.Context, model provider.EmbeddingModelV3, options provider.EmbeddingModelV3CallOptions) (provider.EmbeddingModelV3Result, error) {
	if model == nil {
		return provider.EmbeddingModelV3Result{}, ErrNilModel
	}
	return model.DoEmbed(ctx, options)
}

func GenerateImage(ctx context.Context, model provider.ImageModelV3, options provider.ImageModelV3CallOptions) (provider.ImageModelV3Result, error) {
	if model == nil {
		return provider.ImageModelV3Result{}, ErrNilModel
	}
	return model.DoGenerate(ctx, options)
}

func GenerateSpeech(ctx context.Context, model provider.SpeechModelV3, options provider.SpeechModelV3CallOptions) (provider.SpeechModelV3Result, error) {
	if model == nil {
		return provider.SpeechModelV3Result{}, ErrNilModel
	}
	return model.DoGenerate(ctx, options)
}

func Transcribe(ctx context.Context, model provider.TranscriptionModelV3, options provider.TranscriptionModelV3CallOptions) (provider.TranscriptionModelV3Result, error) {
	if model == nil {
		return provider.TranscriptionModelV3Result{}, ErrNilModel
	}
	return model.DoGenerate(ctx, options)
}

func Rerank(ctx context.Context, model provider.RerankingModelV3, options provider.RerankingModelV3CallOptions) (provider.RerankingModelV3Result, error) {
	if model == nil {
		return provider.RerankingModelV3Result{}, ErrNilModel
	}
	return model.DoRerank(ctx, options)
}
