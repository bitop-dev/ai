package images

import (
	"context"
	"fmt"
	"sync"

	"github.com/bitop-dev/ai/internal/provider"
)

func DefaultMaxImagesPerCall(model string) int {
	switch model {
	case "dall-e-3":
		return 1
	case "dall-e-2":
		return 10
	case "gpt-image-1":
		return 1
	default:
		return 1
	}
}

func GenerateBatched(
	ctx context.Context,
	ip provider.ImageProvider,
	base provider.GenerateImageRequest,
	n int,
	maxPerCall int,
	maxParallel int,
) ([]provider.Image, []string, map[string]any, []byte, error) {
	if n <= 0 {
		return nil, nil, nil, nil, fmt.Errorf("n must be > 0")
	}
	if maxPerCall <= 0 {
		maxPerCall = 1
	}
	if maxPerCall > n {
		maxPerCall = n
	}
	if maxParallel <= 0 {
		maxParallel = 4
	}

	type batch struct{ start, count int }
	var batches []batch
	for remaining, start := n, 0; remaining > 0; {
		c := maxPerCall
		if c > remaining {
			c = remaining
		}
		batches = append(batches, batch{start: start, count: c})
		start += c
		remaining -= c
	}

	outImages := make([]provider.Image, n)
	var warnings []string
	var providerMetadata map[string]any
	var firstRaw []byte

	type batchResult struct {
		start int
		resp  provider.GenerateImageResponse
	}
	sem := make(chan struct{}, maxParallel)
	errCh := make(chan error, len(batches))
	resCh := make(chan batchResult, len(batches))
	var wg sync.WaitGroup

	for _, b := range batches {
		wg.Add(1)
		sem <- struct{}{}
		go func(b batch) {
			defer wg.Done()
			defer func() { <-sem }()

			req := base
			req.N = b.count
			resp, err := ip.GenerateImage(ctx, req)
			if err != nil {
				errCh <- err
				return
			}
			resCh <- batchResult{start: b.start, resp: resp}
		}(b)
	}

	wg.Wait()
	close(errCh)
	close(resCh)

	for err := range errCh {
		if err != nil {
			return nil, nil, nil, nil, err
		}
	}

	for r := range resCh {
		if firstRaw == nil && len(r.resp.RawResponse) > 0 {
			firstRaw = r.resp.RawResponse
		}
		if r.resp.ProviderMetadata != nil {
			providerMetadata = mergeProviderMetadata(providerMetadata, r.start, r.resp.ProviderMetadata)
		}
		if len(r.resp.Warnings) > 0 {
			warnings = append(warnings, r.resp.Warnings...)
		}
		for i, img := range r.resp.Images {
			outImages[r.start+i] = img
		}
	}

	compact := make([]provider.Image, 0, len(outImages))
	for _, img := range outImages {
		if len(img.Bytes) == 0 && img.Base64 == "" {
			continue
		}
		compact = append(compact, img)
	}

	return compact, warnings, providerMetadata, firstRaw, nil
}

func mergeProviderMetadata(dst map[string]any, start int, src map[string]any) map[string]any {
	if dst == nil {
		dst = map[string]any{}
	}
	for pk, pv := range src {
		// Special-case OpenAI images metadata: keep a single "images" array aligned to output.
		if pk == "openai" {
			dstOpenAI, _ := dst["openai"].(map[string]any)
			srcOpenAI, _ := pv.(map[string]any)
			if dstOpenAI == nil {
				dstOpenAI = map[string]any{}
				dst["openai"] = dstOpenAI
			}
			if srcImages, ok := srcOpenAI["images"].([]map[string]any); ok {
				dstImages, _ := dstOpenAI["images"].([]map[string]any)
				if len(dstImages) < start+len(srcImages) {
					nd := make([]map[string]any, start+len(srcImages))
					copy(nd, dstImages)
					dstImages = nd
				}
				for i := range srcImages {
					dstImages[start+i] = srcImages[i]
				}
				dstOpenAI["images"] = dstImages
			}
			continue
		}
		if _, exists := dst[pk]; !exists {
			dst[pk] = pv
		}
	}
	return dst
}
