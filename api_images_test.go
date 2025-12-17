package ai

import (
	"context"
	"sync"
	"testing"

	"github.com/bitop-dev/ai/internal/provider"
)

type fakeImageProvider struct {
	*fakeProvider
	gen func(call int, req provider.GenerateImageRequest) (provider.GenerateImageResponse, error)
	n   int
}

func (p *fakeImageProvider) GenerateImage(ctx context.Context, req provider.GenerateImageRequest) (provider.GenerateImageResponse, error) {
	_ = ctx
	call := p.n
	p.n++
	if p.gen == nil {
		return provider.GenerateImageResponse{}, nil
	}
	return p.gen(call, req)
}

func TestGenerateImage_Batched(t *testing.T) {
	ip := &fakeImageProvider{}
	var wg sync.WaitGroup
	wg.Add(5)
	ip.gen = func(call int, req provider.GenerateImageRequest) (provider.GenerateImageResponse, error) {
		defer wg.Done()
		if req.N != 1 {
			t.Fatalf("expected N=1 for dall-e-3, got %d", req.N)
		}
		return provider.GenerateImageResponse{
			N:      req.N,
			Images: []provider.Image{{Base64: "aGVsbG8="}},
		}, nil
	}
	providerName := registerFakeProvider(t, ip)

	resp, err := GenerateImage(context.Background(), GenerateImageRequest{
		Model:  testModel{provider: providerName, name: "dall-e-3"},
		Prompt: "x",
		N:      5,
		// Force batching + concurrency-limited execution to catch race/close bugs.
		MaxParallelCalls: 2,
	})
	if err != nil {
		t.Fatal(err)
	}
	wg.Wait()
	if len(resp.Images) != 5 {
		t.Fatalf("images=%d", len(resp.Images))
	}
	for _, img := range resp.Images {
		if img.Base64 == "" || len(img.Uint8Array) == 0 {
			t.Fatalf("expected decoded image bytes")
		}
	}
}
