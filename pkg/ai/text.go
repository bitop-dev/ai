package ai

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/bitop-dev/ai/pkg/provider"
	"github.com/bitop-dev/ai/pkg/providerutils"
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
	SchemaValidator  providerutils.SchemaValidator
	Telemetry        Telemetry
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

type StreamObjectResult struct {
	Stream   *Stream[provider.StreamPart]
	Request  *provider.LanguageModelV3Request
	Response *provider.LanguageModelV3Response
	Schema   *providerutils.Schema
}

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

type GenerateObjectResult struct {
	Object           provider.JSONValue
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

func GenerateText(ctx context.Context, model provider.LanguageModelV3, options GenerateTextOptions) (GenerateTextResult, error) {
	if model == nil {
		return GenerateTextResult{}, ErrNilModel
	}

	streamResult, err := streamText(ctx, model, StreamTextOptions(options), TelemetryOperationGenerateText)
	if err != nil {
		return GenerateTextResult{}, err
	}
	defer streamResult.Stream.Close()

	collected, err := collectStreamText(streamResult.Stream)
	if err != nil {
		return GenerateTextResult{}, err
	}

	result := GenerateTextResult{
		Text:             collected.Text,
		Parts:            collected.Parts,
		FinishReason:     collected.FinishReason,
		Usage:            collected.Usage,
		Warnings:         collected.Warnings,
		ResponseMetadata: collected.ResponseMetadata,
		ProviderMetadata: collected.ProviderMetadata,
		Request:          streamResult.Request,
		Response:         streamResult.Response,
	}
	return result, nil
}

func StreamText(ctx context.Context, model provider.LanguageModelV3, options StreamTextOptions) (StreamTextResult, error) {
	return streamText(ctx, model, options, TelemetryOperationStreamText)
}

func streamText(ctx context.Context, model provider.LanguageModelV3, options StreamTextOptions, operation string) (StreamTextResult, error) {
	if model == nil {
		return StreamTextResult{}, ErrNilModel
	}
	ctx, cancel := context.WithCancel(ctx)
	telemetry := options.Telemetry
	if telemetry == nil {
		telemetry = NoopTelemetry{}
	}
	span := telemetry.Start(ctx, TelemetryRequest{
		Operation: operation,
		Provider:  model.ProviderID(),
		Model:     model.ModelID(),
		Metadata:  options.RequestOptions.Metadata,
	})
	startTime := time.Now()
	callOptions := toLanguageModelCallOptions(options)
	streamResult, err := model.DoStream(ctx, callOptions)
	if err != nil {
		cancel()
		span.Error(ctx, TelemetrySpanError{Duration: time.Since(startTime), Err: err})
		return StreamTextResult{}, err
	}
	stream := newStream(ctx, cancel, streamResult.Stream)
	attachTelemetry(stream, span, startTime)

	return StreamTextResult{
		Stream:   stream,
		Request:  streamResult.Request,
		Response: streamResult.Response,
	}, nil
}

func GenerateObject(ctx context.Context, model provider.LanguageModelV3, options GenerateObjectOptions) (GenerateObjectResult, error) {
	options.ResponseFormat = jsonResponseFormat(options.ResponseFormat)
	streamResult, err := streamText(ctx, model, StreamTextOptions(options), TelemetryOperationGenerateObject)
	if err != nil {
		return GenerateObjectResult{}, err
	}
	defer streamResult.Stream.Close()

	collected, err := collectStreamText(streamResult.Stream)
	if err != nil {
		return GenerateObjectResult{}, err
	}

	object, err := parseStructuredOutput(collected.Text, schemaForOptions(options))
	if err != nil {
		return GenerateObjectResult{}, err
	}

	return GenerateObjectResult{
		Object:           object,
		Text:             collected.Text,
		Parts:            collected.Parts,
		FinishReason:     collected.FinishReason,
		Usage:            collected.Usage,
		Warnings:         collected.Warnings,
		ResponseMetadata: collected.ResponseMetadata,
		ProviderMetadata: collected.ProviderMetadata,
		Request:          streamResult.Request,
		Response:         streamResult.Response,
	}, nil
}

func StreamObject(ctx context.Context, model provider.LanguageModelV3, options StreamObjectOptions) (StreamObjectResult, error) {
	options.ResponseFormat = jsonResponseFormat(options.ResponseFormat)
	streamResult, err := streamText(ctx, model, StreamTextOptions(options), TelemetryOperationStreamObject)
	if err != nil {
		return StreamObjectResult{}, err
	}
	return StreamObjectResult{
		Stream:   streamResult.Stream,
		Request:  streamResult.Request,
		Response: streamResult.Response,
		Schema:   schemaForOptions(options),
	}, nil
}

func (result StreamObjectResult) Collect() (GenerateObjectResult, error) {
	if result.Stream == nil {
		return GenerateObjectResult{}, errors.New("stream is nil")
	}
	defer result.Stream.Close()

	collected, err := collectStreamText(result.Stream)
	if err != nil {
		return GenerateObjectResult{}, err
	}

	object, err := parseStructuredOutput(collected.Text, result.Schema)
	if err != nil {
		return GenerateObjectResult{}, err
	}

	return GenerateObjectResult{
		Object:           object,
		Text:             collected.Text,
		Parts:            collected.Parts,
		FinishReason:     collected.FinishReason,
		Usage:            collected.Usage,
		Warnings:         collected.Warnings,
		ResponseMetadata: collected.ResponseMetadata,
		ProviderMetadata: collected.ProviderMetadata,
		Request:          result.Request,
		Response:         result.Response,
	}, nil
}

func jsonResponseFormat(format *provider.ResponseFormat) *provider.ResponseFormat {
	if format == nil {
		return &provider.ResponseFormat{Type: provider.ResponseFormatTypeJSON}
	}
	clone := *format
	clone.Type = provider.ResponseFormatTypeJSON
	return &clone
}

type collectedTextResult struct {
	Text             string
	Parts            []provider.StreamPart
	FinishReason     provider.FinishReason
	Usage            *provider.LanguageModelUsage
	Warnings         []provider.Warning
	ResponseMetadata *provider.ResponseMetadata
	ProviderMetadata provider.ProviderMetadata
}

func collectStreamText(stream *Stream[provider.StreamPart]) (collectedTextResult, error) {
	var result collectedTextResult
	var textBuilder strings.Builder

	for stream.Next() {
		part := stream.Value()
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
				return collectedTextResult{}, part.Error.Err
			}
		}
	}

	if err := stream.Err(); err != nil {
		return collectedTextResult{}, err
	}

	result.Text = textBuilder.String()
	return result, nil
}

func schemaForOptions(options TextOptions) *providerutils.Schema {
	if options.ResponseFormat == nil && options.SchemaValidator == nil {
		return nil
	}
	if options.ResponseFormat == nil {
		if options.SchemaValidator == nil {
			return nil
		}
		return &providerutils.Schema{Validator: options.SchemaValidator}
	}
	if options.ResponseFormat.Schema == nil && options.SchemaValidator == nil {
		return nil
	}
	return &providerutils.Schema{JSONSchema: options.ResponseFormat.Schema, Validator: options.SchemaValidator}
}

func parseStructuredOutput(text string, schema *providerutils.Schema) (provider.JSONValue, error) {
	options := providerutils.ParseOptions{Text: text}
	if schema != nil {
		options.Schema = schema
	}

	value, err := providerutils.ParseJSON(options)
	if err == nil {
		return value, nil
	}
	if _, ok := err.(*providerutils.JSONParseError); !ok {
		return nil, err
	}

	cleaned := extractJSONPayload(text)
	if cleaned == text || cleaned == "" {
		return nil, err
	}

	options.Text = cleaned
	value, fallbackErr := providerutils.ParseJSON(options)
	if fallbackErr != nil {
		return nil, err
	}
	return value, nil
}

func extractJSONPayload(text string) string {
	trimmed := strings.TrimSpace(text)
	if strings.HasPrefix(trimmed, "```") {
		fenceEnd := strings.Index(trimmed, "\n")
		if fenceEnd != -1 {
			closing := strings.LastIndex(trimmed, "```")
			if closing > fenceEnd {
				candidate := strings.TrimSpace(trimmed[fenceEnd+1 : closing])
				if candidate != "" {
					return candidate
				}
			}
		}
	}

	start := strings.IndexAny(trimmed, "[{")
	end := strings.LastIndexAny(trimmed, "]}")
	if start >= 0 && end > start {
		return strings.TrimSpace(trimmed[start : end+1])
	}
	return trimmed
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

func attachTelemetry(stream *Stream[provider.StreamPart], span TelemetrySpan, start time.Time) {
	if stream == nil {
		return
	}
	if span == nil {
		span = NoopSpan{}
	}
	state := &telemetryStreamState{}
	stream.onValue = state.record
	stream.onComplete = func(err error) {
		state.complete(stream.ctx, span, start, err)
	}
}

type telemetryStreamState struct {
	warnings         []provider.Warning
	usage            *provider.LanguageModelUsage
	responseMetadata *provider.ResponseMetadata
	providerMetadata provider.ProviderMetadata
	err              error
}

func (state *telemetryStreamState) record(part provider.StreamPart) {
	if len(part.Warnings) > 0 {
		state.warnings = append(state.warnings, part.Warnings...)
	}
	state.providerMetadata = mergeProviderMetadata(state.providerMetadata, part.ProviderMetadata)
	if part.ResponseMetadata != nil {
		state.responseMetadata = part.ResponseMetadata
		state.providerMetadata = mergeProviderMetadata(state.providerMetadata, part.ResponseMetadata.ProviderMetadata)
	}

	switch part.Type {
	case provider.StreamPartTypeFinish:
		if part.Finish != nil {
			state.usage = part.Finish.Usage
		}
	case provider.StreamPartTypeError:
		if part.Error != nil && part.Error.Err != nil {
			state.err = part.Error.Err
		}
	}
}

func (state *telemetryStreamState) complete(ctx context.Context, span TelemetrySpan, start time.Time, streamErr error) {
	if span == nil {
		return
	}
	err := state.err
	if err == nil {
		err = streamErr
	}
	if err != nil {
		span.Error(ctx, TelemetrySpanError{Duration: time.Since(start), Err: err, Warnings: state.warnings})
		return
	}
	span.End(ctx, TelemetrySpanEnd{
		Duration:         time.Since(start),
		Usage:            state.usage,
		Warnings:         state.warnings,
		ResponseMetadata: state.responseMetadata,
		ProviderMetadata: state.providerMetadata,
	})
}
