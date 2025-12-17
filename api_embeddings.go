package ai

import (
	"context"
	"fmt"
	"time"

	internalEmbeddings "github.com/bitop-dev/ai/internal/embeddings"
	"github.com/bitop-dev/ai/internal/provider"
)

type EmbedRequest struct {
	Model ModelRef
	Input string

	Metadata map[string]string

	Headers    map[string]string
	MaxRetries *int
	Timeout    time.Duration

	ProviderOptions map[string]any
}

type EmbedResponse struct {
	Vector []float32
	Usage  Usage

	RawResponse []byte
}

type EmbedManyRequest struct {
	Model ModelRef
	Input []string

	Metadata map[string]string

	Headers          map[string]string
	MaxRetries       *int
	Timeout          time.Duration
	MaxParallelCalls int

	ProviderOptions map[string]any
}

type EmbedManyResponse struct {
	Vectors [][]float32
	Usage   Usage

	RawResponse []byte
}

func Embed(ctx context.Context, req EmbedRequest) (*EmbedResponse, error) {
	ctx, cancel := applyTimeout(ctx, req.Timeout)
	defer cancel()

	if req.Input == "" {
		return nil, fmt.Errorf("input is required")
	}
	resp, err := EmbedMany(ctx, EmbedManyRequest{
		Model:           req.Model,
		Input:           []string{req.Input},
		Metadata:        req.Metadata,
		Headers:         req.Headers,
		MaxRetries:      req.MaxRetries,
		Timeout:         req.Timeout,
		ProviderOptions: req.ProviderOptions,
	})
	if err != nil {
		return nil, err
	}
	if len(resp.Vectors) != 1 {
		return nil, fmt.Errorf("expected 1 embedding, got %d", len(resp.Vectors))
	}
	return &EmbedResponse{Vector: resp.Vectors[0], Usage: resp.Usage, RawResponse: resp.RawResponse}, nil
}

func EmbedMany(ctx context.Context, req EmbedManyRequest) (*EmbedManyResponse, error) {
	ctx, cancel := applyTimeout(ctx, req.Timeout)
	defer cancel()

	p, err := providerForModel(req.Model)
	if err != nil {
		return nil, err
	}
	ep, ok := p.(provider.EmbeddingProvider)
	if !ok {
		return nil, fmt.Errorf("provider %q does not support embeddings", req.Model.Provider())
	}
	if len(req.Input) == 0 {
		return nil, fmt.Errorf("input is required")
	}

	preq := provider.EmbeddingRequest{
		Model:           req.Model.Name(),
		Inputs:          append([]string(nil), req.Input...),
		Metadata:        cloneStringMap(req.Metadata),
		Headers:         cloneStringMap(req.Headers),
		MaxRetries:      req.MaxRetries,
		ProviderOptions: req.ProviderOptions,
		ProviderData:    nil,
	}
	// Reuse provider-specific wiring if present (e.g. client-bound model ref).
	if c, ok := openAIClientFromModel(req.Model); ok {
		preq.ProviderData = c
	}

	if req.MaxParallelCalls <= 1 || len(req.Input) <= 1 {
		out, err := internalEmbeddings.EmbedMany(ctx, ep, preq, 1)
		if err != nil {
			return nil, mapProviderError(err)
		}
		return &EmbedManyResponse{Vectors: out.Vectors, Usage: Usage{PromptTokens: out.Usage.PromptTokens, CompletionTokens: out.Usage.CompletionTokens, TotalTokens: out.Usage.TotalTokens}, RawResponse: out.RawResponse}, nil
	}

	out, err := internalEmbeddings.EmbedMany(ctx, ep, preq, req.MaxParallelCalls)
	if err != nil {
		return nil, mapProviderError(err)
	}
	return &EmbedManyResponse{Vectors: out.Vectors, Usage: Usage{PromptTokens: out.Usage.PromptTokens, CompletionTokens: out.Usage.CompletionTokens, TotalTokens: out.Usage.TotalTokens}, RawResponse: out.RawResponse}, nil
}
