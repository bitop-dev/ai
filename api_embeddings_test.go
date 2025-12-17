package ai

import (
	"context"
	"testing"

	"github.com/bitop-dev/ai/internal/provider"
)

func TestEmbedMany_ProviderNotSupported(t *testing.T) {
	fp := &fakeProvider{}
	fp.generate = func(call int, req provider.Request) (provider.Response, error) {
		_ = call
		_ = req
		return provider.Response{}, nil
	}
	providerName := registerFakeProvider(t, fp)

	_, err := EmbedMany(context.Background(), EmbedManyRequest{
		Model: testModel{provider: providerName, name: "m"},
		Input: []string{"a"},
	})
	if err == nil {
		t.Fatalf("expected error")
	}
}

type fakeEmbeddingProvider struct {
	*fakeProvider
	embed func(call int, req provider.EmbeddingRequest) (provider.EmbeddingResponse, error)
	n     int
}

func (p *fakeEmbeddingProvider) Embed(ctx context.Context, req provider.EmbeddingRequest) (provider.EmbeddingResponse, error) {
	_ = ctx
	call := p.n
	p.n++
	if p.embed == nil {
		return provider.EmbeddingResponse{}, nil
	}
	return p.embed(call, req)
}

func TestEmbedMany_Success(t *testing.T) {
	ep := &fakeEmbeddingProvider{}
	ep.embed = func(call int, req provider.EmbeddingRequest) (provider.EmbeddingResponse, error) {
		_ = call
		if req.Model != "text-embedding-test" {
			t.Fatalf("model=%q", req.Model)
		}
		if len(req.Inputs) != 2 || req.Inputs[0] != "a" || req.Inputs[1] != "b" {
			t.Fatalf("inputs=%#v", req.Inputs)
		}
		return provider.EmbeddingResponse{
			Vectors: [][]float32{{1, 2}, {3, 4}},
			Usage:   provider.Usage{PromptTokens: 10, TotalTokens: 10},
		}, nil
	}
	providerName := registerFakeProvider(t, ep)

	resp, err := EmbedMany(context.Background(), EmbedManyRequest{
		Model: testModel{provider: providerName, name: "text-embedding-test"},
		Input: []string{"a", "b"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.Vectors) != 2 || len(resp.Vectors[0]) != 2 {
		t.Fatalf("vectors=%#v", resp.Vectors)
	}
	if resp.Usage.TotalTokens != 10 {
		t.Fatalf("usage=%#v", resp.Usage)
	}
}

func TestEmbedMany_ParallelPreservesOrder(t *testing.T) {
	ep := &fakeEmbeddingProvider{}
	ep.embed = func(call int, req provider.EmbeddingRequest) (provider.EmbeddingResponse, error) {
		vecs := make([][]float32, len(req.Inputs))
		for i := range req.Inputs {
			// Input values are "v0", "v1", ...
			n := int(req.Inputs[i][1] - '0')
			vecs[i] = []float32{float32(n)}
		}
		// Ensure both batches got called.
		if call == 0 && len(req.Inputs) == 0 {
			t.Fatalf("unexpected empty batch")
		}
		return provider.EmbeddingResponse{
			Vectors: vecs,
			Usage:   provider.Usage{PromptTokens: 1, TotalTokens: 1},
		}, nil
	}
	providerName := registerFakeProvider(t, ep)

	resp, err := EmbedMany(context.Background(), EmbedManyRequest{
		Model:            testModel{provider: providerName, name: "text-embedding-test"},
		Input:            []string{"v0", "v1", "v2", "v3"},
		MaxParallelCalls: 2,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.Vectors) != 4 {
		t.Fatalf("vectors=%#v", resp.Vectors)
	}
	for i := 0; i < 4; i++ {
		if got := int(resp.Vectors[i][0]); got != i {
			t.Fatalf("index %d got %d", i, got)
		}
	}
}
