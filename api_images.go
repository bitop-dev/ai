package ai

import (
	"context"
	"encoding/base64"
	"fmt"
	"time"

	internalImages "github.com/bitop-dev/ai/internal/images"
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
	Timeout    time.Duration

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
	ctx, cancel := applyTimeout(ctx, req.Timeout)
	defer cancel()

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
		maxPerCall = internalImages.DefaultMaxImagesPerCall(req.Model.Name())
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

	provImages, warnings, metadata, raw, err := internalImages.GenerateBatched(ctx, ip, preqBase, n, maxPerCall, maxParallel)
	if err != nil {
		return nil, mapProviderError(err)
	}
	if len(provImages) == 0 {
		return nil, &NoImageGeneratedError{Provider: req.Model.Provider(), RawResponse: raw}
	}

	images := make([]Image, len(provImages))
	for i, img := range provImages {
		images[i] = fromProviderImage(img)
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
