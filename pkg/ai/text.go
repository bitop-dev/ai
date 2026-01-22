package ai

import (
	"context"
	"errors"
	"strings"

	"github.com/vercel/ai-sdk-go/pkg/provider"
)

var ErrNilModel = errors.New("model is nil")

type TextOptions struct {
	Prompt           provider.Prompt
	MaxOutputTokens  *int
	Temperature      *float64
	StopSequences    []string
	TopP             *float64
	TopK             *int
	PresencePenalty  *float64
	FrequencyPenalty *float64
	ResponseFormat   *provider.ResponseFormat
	Seed             *int
	ToolChoice       *provider.ToolChoice
	IncludeRawChunks *bool
	RequestOptions   provider.RequestOptions
}

type GenerateTextOptions = TextOptions
type StreamTextOptions = TextOptions
type GenerateObjectOptions = TextOptions
type StreamObjectOptions = TextOptions

type StreamTextResult struct {
	Stream   *Stream[provider.StreamPart]
	Request  *provider.LanguageModelV3Request
	Response *provider.LanguageModelV3Response
}

type StreamObjectResult = StreamTextResult

type GenerateTextResult struct {
	Text             string
	Parts            []provider.StreamPart
	FinishReason     provider.FinishReason
	Usage            *provider.LanguageModelUsage
	Warnings         []provider.Warning
	ResponseMetadata *provider.ResponseMetadata
	ProviderMetadata provider.ProviderMetadata
	Request          *provider.LanguageModelV3Request
	Response         *provider.LanguageModelV3Response
}

type GenerateObjectResult = GenerateTextResult

func GenerateText(ctx context.Context, model provider.LanguageModelV3, options GenerateTextOptions) (GenerateTextResult, error) {
	if model == nil {
		return GenerateTextResult{}, ErrNilModel
	}

	streamResult, err := StreamText(ctx, model, StreamTextOptions(options))
	if err != nil {
		return GenerateTextResult{}, err
	}
	defer streamResult.Stream.Close()

	result := GenerateTextResult{
		Request:  streamResult.Request,
		Response: streamResult.Response,
	}
	var textBuilder strings.Builder

	for streamResult.Stream.Next() {
		part := streamResult.Stream.Value()
		result.Parts = append(result.Parts, part)
		result.Warnings = append(result.Warnings, part.Warnings...)
		result.ProviderMetadata = mergeProviderMetadata(result.ProviderMetadata, part.ProviderMetadata)
		if part.ResponseMetadata != nil {
			result.ResponseMetadata = part.ResponseMetadata
			result.ProviderMetadata = mergeProviderMetadata(result.ProviderMetadata, part.ResponseMetadata.ProviderMetadata)
		}

		switch part.Type {
		case provider.StreamPartTypeTextStart:
			if part.TextStart != nil {
				textBuilder.WriteString(part.TextStart.Text)
			}
		case provider.StreamPartTypeTextDelta:
			if part.TextDelta != nil {
				textBuilder.WriteString(part.TextDelta.Delta)
			}
		case provider.StreamPartTypeTextEnd:
			if part.TextEnd != nil {
				textBuilder.WriteString(part.TextEnd.Text)
			}
		case provider.StreamPartTypeFinish:
			if part.Finish != nil {
				result.FinishReason = part.Finish.Reason
				result.Usage = part.Finish.Usage
			}
		case provider.StreamPartTypeError:
			if part.Error != nil && part.Error.Err != nil {
				return GenerateTextResult{}, part.Error.Err
			}
		}
	}

	if err := streamResult.Stream.Err(); err != nil {
		return GenerateTextResult{}, err
	}

	result.Text = textBuilder.String()
	return result, nil
}

func StreamText(ctx context.Context, model provider.LanguageModelV3, options StreamTextOptions) (StreamTextResult, error) {
	if model == nil {
		return StreamTextResult{}, ErrNilModel
	}
	ctx, cancel := context.WithCancel(ctx)
	callOptions := toLanguageModelCallOptions(options)
	streamResult, err := model.DoStream(ctx, callOptions)
	if err != nil {
		cancel()
		return StreamTextResult{}, err
	}

	return StreamTextResult{
		Stream:   newStream(ctx, cancel, streamResult.Stream),
		Request:  streamResult.Request,
		Response: streamResult.Response,
	}, nil
}

func GenerateObject(ctx context.Context, model provider.LanguageModelV3, options GenerateObjectOptions) (GenerateObjectResult, error) {
	options.ResponseFormat = jsonResponseFormat(options.ResponseFormat)
	return GenerateText(ctx, model, GenerateTextOptions(options))
}

func StreamObject(ctx context.Context, model provider.LanguageModelV3, options StreamObjectOptions) (StreamObjectResult, error) {
	options.ResponseFormat = jsonResponseFormat(options.ResponseFormat)
	return StreamText(ctx, model, StreamTextOptions(options))
}

func jsonResponseFormat(format *provider.ResponseFormat) *provider.ResponseFormat {
	if format == nil {
		return &provider.ResponseFormat{Type: provider.ResponseFormatTypeJSON}
	}
	clone := *format
	clone.Type = provider.ResponseFormatTypeJSON
	return &clone
}

func toLanguageModelCallOptions(options TextOptions) provider.LanguageModelV3CallOptions {
	callOptions := provider.LanguageModelV3CallOptions{
		Prompt:          options.Prompt,
		StopSequences:   options.StopSequences,
		ResponseFormat:  options.ResponseFormat,
		ToolChoice:      options.ToolChoice,
		RequestOptions:  options.RequestOptions,
		ProviderOptions: options.RequestOptions.ProviderOptions,
	}

	if options.MaxOutputTokens != nil {
		callOptions.MaxOutputTokens = *options.MaxOutputTokens
	}
	if options.Temperature != nil {
		callOptions.Temperature = *options.Temperature
	}
	if options.TopP != nil {
		callOptions.TopP = *options.TopP
	}
	if options.TopK != nil {
		callOptions.TopK = *options.TopK
	}
	if options.PresencePenalty != nil {
		callOptions.PresencePenalty = *options.PresencePenalty
	}
	if options.FrequencyPenalty != nil {
		callOptions.FrequencyPenalty = *options.FrequencyPenalty
	}
	if options.Seed != nil {
		callOptions.Seed = *options.Seed
	}
	if options.IncludeRawChunks != nil {
		callOptions.IncludeRawChunks = *options.IncludeRawChunks
	}

	return callOptions
}

func mergeProviderMetadata(dst provider.ProviderMetadata, src provider.ProviderMetadata) provider.ProviderMetadata {
	if len(src) == 0 {
		return dst
	}
	if dst == nil {
		dst = provider.ProviderMetadata{}
	}
	for providerID, metadata := range src {
		if existing, ok := dst[providerID]; ok {
			if existing == nil {
				existing = map[string]any{}
			}
			for key, value := range metadata {
				existing[key] = value
			}
			dst[providerID] = existing
			continue
		}
		copied := map[string]any{}
		for key, value := range metadata {
			copied[key] = value
		}
		dst[providerID] = copied
	}
	return dst
}
