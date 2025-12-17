package embeddings

import (
	"context"
	"fmt"
	"sync"

	"github.com/bitop-dev/ai/internal/provider"
	"github.com/bitop-dev/ai/internal/tools"
)

func EmbedMany(ctx context.Context, ep provider.EmbeddingProvider, req provider.EmbeddingRequest, maxParallel int) (provider.EmbeddingResponse, error) {
	if len(req.Inputs) == 0 {
		return provider.EmbeddingResponse{}, fmt.Errorf("input is required")
	}
	if maxParallel <= 1 || len(req.Inputs) <= 1 {
		return ep.Embed(ctx, req)
	}
	if maxParallel < 2 {
		maxParallel = 2
	}
	if maxParallel > len(req.Inputs) {
		maxParallel = len(req.Inputs)
	}

	type batch struct{ start, end int }
	rawBatches := splitIntoBatches(len(req.Inputs), maxParallel)
	batches := make([]batch, len(rawBatches))
	for i, b := range rawBatches {
		batches[i] = batch{start: b.start, end: b.end}
	}

	outVectors := make([][]float32, len(req.Inputs))
	var aggUsage provider.Usage

	var firstRaw []byte
	var firstRawOnce sync.Once

	var mu sync.Mutex
	var wg sync.WaitGroup
	errCh := make(chan error, len(batches))

	for _, b := range batches {
		wg.Add(1)
		go func(b batch) {
			defer wg.Done()

			subReq := req
			subReq.Inputs = append([]string(nil), req.Inputs[b.start:b.end]...)

			resp, err := ep.Embed(ctx, subReq)
			if err != nil {
				errCh <- err
				return
			}
			if len(resp.Vectors) != len(subReq.Inputs) {
				errCh <- fmt.Errorf("embedding response count mismatch: got %d want %d", len(resp.Vectors), len(subReq.Inputs))
				return
			}

			mu.Lock()
			for i := range resp.Vectors {
				outVectors[b.start+i] = resp.Vectors[i]
			}
			aggUsage = tools.AddUsage(aggUsage, resp.Usage)
			mu.Unlock()

			firstRawOnce.Do(func() { firstRaw = resp.RawResponse })
		}(b)
	}

	wg.Wait()
	close(errCh)

	for err := range errCh {
		if err != nil {
			return provider.EmbeddingResponse{}, err
		}
	}

	return provider.EmbeddingResponse{
		Vectors:     outVectors,
		Usage:       aggUsage,
		RawResponse: firstRaw,
	}, nil
}

func splitIntoBatches(n, parts int) []struct{ start, end int } {
	if parts <= 0 {
		parts = 1
	}
	if parts > n {
		parts = n
	}
	out := make([]struct{ start, end int }, 0, parts)
	base := n / parts
	rem := n % parts

	start := 0
	for i := 0; i < parts; i++ {
		size := base
		if i < rem {
			size++
		}
		end := start + size
		out = append(out, struct{ start, end int }{start: start, end: end})
		start = end
	}
	return out
}
