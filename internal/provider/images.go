package provider

import "context"

type ImageProvider interface {
	GenerateImage(ctx context.Context, req GenerateImageRequest) (GenerateImageResponse, error)
}

type Image struct {
	Base64    string
	Bytes     []byte
	MediaType string
}

type GenerateImageRequest struct {
	Model  string
	Prompt string

	Size        string
	AspectRatio string

	N    int
	Seed *int64

	Headers    map[string]string
	MaxRetries *int

	ProviderOptions any
	ProviderData    any
}

type GenerateImageResponse struct {
	N      int
	Images []Image

	Warnings         []string
	ProviderMetadata map[string]any

	RawResponse []byte
}
