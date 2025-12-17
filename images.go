package ai

import (
	"context"
	"encoding/base64"
	"fmt"
	"sync"

	"github.com/bitop-dev/ai/internal/provider"
)

type Image struct {
	Base64     string
	Uint8Array []byte
	MediaType  string
}

type GenerateImageRequest struct {
	Model  ModelRef
	Prompt string

	Size        string
	AspectRatio string

	N                int
	MaxImagesPerCall int
	MaxParallelCalls int
	Seed             *int64

	Headers    map[string]string
	MaxRetries *int

	ProviderOptions map[string]any
}

type GenerateImageResponse struct {
	Image  Image
	Images []Image

	Warnings         []string
	ProviderMetadata map[string]any

	RawResponse []byte
}

type NoImageGeneratedError struct {
	Provider    string
	Cause       error
	RawResponse []byte
}

func (e *NoImageGeneratedError) Error() string {
	if e == nil {
		return ""
	}
	if e.Cause != nil {
		return fmt.Sprintf("%s: no image generated: %v", e.Provider, e.Cause)
	}
	return fmt.Sprintf("%s: no image generated", e.Provider)
}

func (e *NoImageGeneratedError) Unwrap() error { return e.Cause }

func IsNoImageGenerated(err error) bool {
	_, ok := err.(*NoImageGeneratedError)
	return ok
}

func GenerateImage(ctx context.Context, req GenerateImageRequest) (*GenerateImageResponse, error) {
	if req.Prompt == "" {
		return nil, fmt.Errorf("prompt is required")
	}
	p, err := providerForModel(req.Model)
	if err != nil {
		return nil, err
	}
	ip, ok := p.(provider.ImageProvider)
	if !ok {
		return nil, fmt.Errorf("provider %q does not support image generation", req.Model.Provider())
	}

	n := req.N
	if n <= 0 {
		n = 1
	}
	maxPerCall := req.MaxImagesPerCall
	if maxPerCall <= 0 {
		maxPerCall = defaultMaxImagesPerCall(req.Model.Name())
	}
	if maxPerCall <= 0 {
		maxPerCall = 1
	}
	if maxPerCall > n {
		maxPerCall = n
	}

	maxParallel := req.MaxParallelCalls
	if maxParallel <= 0 {
		maxParallel = 4
	}

	preqBase := provider.GenerateImageRequest{
		Model:           req.Model.Name(),
		Prompt:          req.Prompt,
		Size:            req.Size,
		AspectRatio:     req.AspectRatio,
		Seed:            req.Seed,
		Headers:         cloneStringMap(req.Headers),
		MaxRetries:      req.MaxRetries,
		ProviderOptions: req.ProviderOptions,
		ProviderData:    nil,
	}
	if c, ok := openAIClientFromModel(req.Model); ok {
		preqBase.ProviderData = c
	}

	images, warnings, metadata, raw, err := generateImagesBatched(ctx, ip, preqBase, n, maxPerCall, maxParallel)
	if err != nil {
		return nil, err
	}
	if len(images) == 0 {
		return nil, &NoImageGeneratedError{Provider: req.Model.Provider(), RawResponse: raw}
	}

	out := &GenerateImageResponse{
		Images:           images,
		Warnings:         warnings,
		ProviderMetadata: metadata,
		RawResponse:      raw,
	}
	out.Image = images[0]
	return out, nil
}

func generateImagesBatched(
	ctx context.Context,
	ip provider.ImageProvider,
	base provider.GenerateImageRequest,
	n int,
	maxPerCall int,
	maxParallel int,
) ([]Image, []string, map[string]any, []byte, error) {
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

	outImages := make([]Image, n)
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
				errCh <- mapProviderError(err)
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
		if len(r.resp.Images) != r.resp.N {
			// Allow provider to omit N, but images must be present.
		}
		for i, img := range r.resp.Images {
			outImages[r.start+i] = fromProviderImage(img)
		}
	}

	// Drop any empty slots if provider returned fewer images than requested.
	compact := make([]Image, 0, len(outImages))
	for _, img := range outImages {
		if len(img.Uint8Array) == 0 && img.Base64 == "" {
			continue
		}
		compact = append(compact, img)
	}

	return compact, warnings, providerMetadata, firstRaw, nil
}

func fromProviderImage(img provider.Image) Image {
	out := Image{
		Base64:    img.Base64,
		MediaType: img.MediaType,
	}
	if out.MediaType == "" {
		out.MediaType = "image/png"
	}
	if len(img.Bytes) > 0 {
		out.Uint8Array = append([]byte(nil), img.Bytes...)
		return out
	}
	if img.Base64 != "" {
		if b, err := base64.StdEncoding.DecodeString(img.Base64); err == nil {
			out.Uint8Array = b
		}
	}
	return out
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
				// Ensure dstImages is large enough and fill at correct indices.
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
		// Default: first-wins.
		if _, exists := dst[pk]; !exists {
			dst[pk] = pv
		}
	}
	return dst
}

func defaultMaxImagesPerCall(model string) int {
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
