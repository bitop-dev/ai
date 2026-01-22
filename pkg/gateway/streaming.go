package gateway

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/vercel/ai-sdk-go/pkg/provider"
	"github.com/vercel/ai-sdk-go/pkg/providerutils"
)

const gatewayLanguageModelPath = "/language-model"

func (m *GatewayLanguageModel) streamLanguageModel(ctx context.Context, options provider.LanguageModelV3CallOptions) (provider.LanguageModelV3StreamResult, error) {
	payload, err := buildGatewayLanguageModelRequest(options)
	if err != nil {
		return provider.LanguageModelV3StreamResult{}, err
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return provider.LanguageModelV3StreamResult{}, err
	}

	headers := map[string]string{
		"Content-Type": "application/json",
		"ai-language-model-specification-version": "3",
		"ai-language-model-id":                    string(m.modelID),
		"ai-language-model-streaming":             "true",
	}
	for key, value := range m.headers {
		headers[key] = value
	}

	url := m.baseURL + gatewayLanguageModelPath
	request, cancel, err := providerutils.BuildRequest(ctx, http.MethodPost, url, bytes.NewReader(body), headers, options.RequestOptions)
	if err != nil {
		if cancel != nil {
			cancel()
		}
		return provider.LanguageModelV3StreamResult{}, err
	}

	response, err := m.httpClient.Do(request)
	if err != nil {
		if cancel != nil {
			cancel()
		}
		return provider.LanguageModelV3StreamResult{}, mapGatewayRequestError(err, m.providerID, m.modelID, nil)
	}

	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		responseBody, readErr := readGatewayResponseBody(response)
		if cancel != nil {
			cancel()
		}
		if readErr != nil {
			return provider.LanguageModelV3StreamResult{}, mapGatewayRequestError(readErr, m.providerID, m.modelID, response)
		}
		return provider.LanguageModelV3StreamResult{}, mapGatewayResponseError(response, responseBody, m.providerID, m.modelID)
	}

	stream := make(chan provider.StreamPart)
	go func() {
		defer func() {
			_ = response.Body.Close()
			if cancel != nil {
				cancel()
			}
			close(stream)
		}()

		parseErr := providerutils.ParseSSE(ctx, response.Body, providerutils.SSEParseOptions{
			OnEvent: func(event providerutils.SSEEvent) error {
				data := strings.TrimSpace(event.Data)
				if data == "" || data == "[DONE]" {
					return nil
				}

				var payload map[string]any
				if err := json.Unmarshal([]byte(data), &payload); err != nil {
					return err
				}

				part, ok, err := mapGatewayStreamPart(payload, options.IncludeRawChunks, m.providerID, m.modelID)
				if err != nil {
					return err
				}
				if ok {
					stream <- part
				}
				return nil
			},
		})
		if parseErr != nil && !errors.Is(parseErr, context.Canceled) {
			stream <- provider.StreamPart{
				Type:  provider.StreamPartTypeError,
				Error: &provider.StreamError{Err: parseErr},
			}
		}
	}()

	return provider.LanguageModelV3StreamResult{
		Stream: stream,
		Request: &provider.LanguageModelV3Request{
			Body: payload,
		},
		Response: &provider.LanguageModelV3Response{
			Headers: response.Header.Clone(),
		},
	}, nil
}

func buildGatewayLanguageModelRequest(options provider.LanguageModelV3CallOptions) (map[string]any, error) {
	encodedPrompt, err := encodeGatewayPrompt(options.Prompt)
	if err != nil {
		return nil, err
	}

	payload := map[string]any{
		"prompt": encodedPrompt,
	}

	if options.MaxOutputTokens > 0 {
		payload["maxOutputTokens"] = options.MaxOutputTokens
	}
	if options.Temperature != 0 {
		payload["temperature"] = options.Temperature
	}
	if len(options.StopSequences) > 0 {
		payload["stopSequences"] = options.StopSequences
	}
	if options.TopP != 0 {
		payload["topP"] = options.TopP
	}
	if options.TopK != 0 {
		payload["topK"] = options.TopK
	}
	if options.PresencePenalty != 0 {
		payload["presencePenalty"] = options.PresencePenalty
	}
	if options.FrequencyPenalty != 0 {
		payload["frequencyPenalty"] = options.FrequencyPenalty
	}
	if options.ResponseFormat != nil {
		payload["responseFormat"] = options.ResponseFormat
	}
	if options.Seed != 0 {
		payload["seed"] = options.Seed
	}
	if options.ToolChoice != nil {
		payload["toolChoice"] = options.ToolChoice
	}
	if options.ProviderOptions != nil {
		payload["providerOptions"] = options.ProviderOptions
	}

	return payload, nil
}

func encodeGatewayPrompt(prompt provider.Prompt) ([]map[string]any, error) {
	encoded := make([]map[string]any, 0, len(prompt.Messages))
	for _, message := range prompt.Messages {
		content, err := encodeGatewayContent(message.Content)
		if err != nil {
			return nil, err
		}
		entry := map[string]any{
			"role":    message.Role,
			"content": content,
		}
		if message.Name != "" {
			entry["name"] = message.Name
		}
		if message.ToolCallID != "" {
			entry["toolCallId"] = message.ToolCallID
		}
		encoded = append(encoded, entry)
	}
	return encoded, nil
}

func encodeGatewayContent(parts []provider.ContentPart) ([]map[string]any, error) {
	encoded := make([]map[string]any, 0, len(parts))
	for _, part := range parts {
		entry, err := encodeGatewayContentPart(part)
		if err != nil {
			return nil, err
		}
		encoded = append(encoded, entry)
	}
	return encoded, nil
}

func encodeGatewayContentPart(part provider.ContentPart) (map[string]any, error) {
	switch typed := part.(type) {
	case provider.TextContent:
		return map[string]any{"type": typed.ContentType(), "text": typed.Text}, nil
	case *provider.TextContent:
		return map[string]any{"type": typed.ContentType(), "text": typed.Text}, nil
	case provider.ToolCallContent:
		return map[string]any{"type": typed.ContentType(), "toolCall": typed.ToolCall}, nil
	case *provider.ToolCallContent:
		return map[string]any{"type": typed.ContentType(), "toolCall": typed.ToolCall}, nil
	case provider.ToolResultContent:
		return map[string]any{"type": typed.ContentType(), "toolResult": typed.ToolResult}, nil
	case *provider.ToolResultContent:
		return map[string]any{"type": typed.ContentType(), "toolResult": typed.ToolResult}, nil
	case provider.SourceContent:
		return map[string]any{"type": typed.ContentType(), "source": typed.Source}, nil
	case *provider.SourceContent:
		return map[string]any{"type": typed.ContentType(), "source": typed.Source}, nil
	case provider.ReasoningContent:
		return map[string]any{"type": typed.ContentType(), "text": typed.Text}, nil
	case *provider.ReasoningContent:
		return map[string]any{"type": typed.ContentType(), "text": typed.Text}, nil
	case provider.ImageContent:
		value, err := encodeGatewayBinaryContent(typed.URL, typed.Data, typed.MediaType)
		if err != nil {
			return nil, err
		}
		return map[string]any{"type": typed.ContentType(), "data": value, "mediaType": typed.MediaType}, nil
	case *provider.ImageContent:
		value, err := encodeGatewayBinaryContent(typed.URL, typed.Data, typed.MediaType)
		if err != nil {
			return nil, err
		}
		return map[string]any{"type": typed.ContentType(), "data": value, "mediaType": typed.MediaType}, nil
	case provider.FileContent:
		value, err := encodeGatewayBinaryContent(typed.URL, typed.Data, typed.MediaType)
		if err != nil {
			return nil, err
		}
		payload := map[string]any{"type": typed.ContentType(), "data": value, "mediaType": typed.MediaType}
		if typed.Name != "" {
			payload["name"] = typed.Name
		}
		return payload, nil
	case *provider.FileContent:
		value, err := encodeGatewayBinaryContent(typed.URL, typed.Data, typed.MediaType)
		if err != nil {
			return nil, err
		}
		payload := map[string]any{"type": typed.ContentType(), "data": value, "mediaType": typed.MediaType}
		if typed.Name != "" {
			payload["name"] = typed.Name
		}
		return payload, nil
	default:
		return nil, fmt.Errorf("unsupported content part %T", part)
	}
}

func encodeGatewayBinaryContent(url string, data []byte, mediaType string) (string, error) {
	if url != "" {
		return url, nil
	}
	if len(data) == 0 {
		return "", nil
	}
	if mediaType == "" {
		mediaType = "application/octet-stream"
	}
	encoded := base64.StdEncoding.EncodeToString(data)
	return fmt.Sprintf("data:%s;base64,%s", mediaType, encoded), nil
}

func mapGatewayStreamPart(payload map[string]any, includeRaw bool, providerID provider.ProviderID, modelID provider.ModelID) (provider.StreamPart, bool, error) {
	partType := toString(payload["type"])
	if partType == "" {
		return provider.StreamPart{}, false, errors.New("missing stream part type")
	}

	var part provider.StreamPart
	switch partType {
	case "stream-start":
		part = provider.StreamPart{
			Type:        provider.StreamPartTypeStreamStart,
			StreamStart: &provider.StreamStart{ProviderID: providerID, ModelID: modelID},
		}
	case "text-start":
		part = provider.StreamPart{
			Type:      provider.StreamPartTypeTextStart,
			TextStart: &provider.TextStart{Text: toString(payload["text"])},
		}
	case "text-delta":
		part = provider.StreamPart{
			Type:      provider.StreamPartTypeTextDelta,
			TextDelta: &provider.TextDelta{Delta: coalesceString(payload["textDelta"], payload["delta"])},
		}
	case "text-end":
		part = provider.StreamPart{
			Type:    provider.StreamPartTypeTextEnd,
			TextEnd: &provider.TextEnd{Text: toString(payload["text"])},
		}
	case "tool-input-start":
		part = provider.StreamPart{
			Type: provider.StreamPartTypeToolInputStart,
			ToolInputStart: &provider.ToolInputStart{
				ToolCallID: toString(payload["toolCallId"]),
				Name:       toString(payload["toolName"]),
			},
		}
	case "tool-input-delta":
		part = provider.StreamPart{
			Type: provider.StreamPartTypeToolInputDelta,
			ToolInputDelta: &provider.ToolInputDelta{
				ToolCallID: toString(payload["toolCallId"]),
				Delta:      coalesceString(payload["textDelta"], payload["delta"]),
			},
		}
	case "tool-input-end":
		part = provider.StreamPart{
			Type: provider.StreamPartTypeToolInputEnd,
			ToolInputEnd: &provider.ToolInputEnd{
				ToolCallID: toString(payload["toolCallId"]),
			},
		}
	case "tool-call":
		call, err := decodeToolCall(payload["toolCall"])
		if err != nil {
			return provider.StreamPart{}, false, err
		}
		part = provider.StreamPart{Type: provider.StreamPartTypeToolCall, ToolCall: &call}
	case "tool-result":
		result, err := decodeToolResult(payload["toolResult"])
		if err != nil {
			return provider.StreamPart{}, false, err
		}
		part = provider.StreamPart{Type: provider.StreamPartTypeToolResult, ToolResult: &result}
	case "source":
		source, err := decodeSource(payload["source"])
		if err != nil {
			return provider.StreamPart{}, false, err
		}
		part = provider.StreamPart{Type: provider.StreamPartTypeSource, Source: &source}
	case "reasoning-start":
		part = provider.StreamPart{
			Type:           provider.StreamPartTypeReasoningStart,
			ReasoningStart: &provider.ReasoningStart{Text: toString(payload["text"])},
		}
	case "reasoning-delta":
		part = provider.StreamPart{
			Type:           provider.StreamPartTypeReasoningDelta,
			ReasoningDelta: &provider.ReasoningDelta{Delta: coalesceString(payload["textDelta"], payload["delta"])},
		}
	case "reasoning-end":
		part = provider.StreamPart{
			Type:         provider.StreamPartTypeReasoningEnd,
			ReasoningEnd: &provider.ReasoningEnd{Text: toString(payload["text"])},
		}
	case "response-metadata":
		part = provider.StreamPart{
			Type: provider.StreamPartTypeResponseMetadata,
			ResponseMetadata: &provider.ResponseMetadata{
				RequestID:  coalesceString(payload["requestId"], payload["requestID"]),
				HTTPStatus: toInt(payload["httpStatus"]),
				Headers:    parseHeaderMap(payload["headers"]),
			},
		}
	case "finish":
		usage := parseUsage(payload["usage"])
		part = provider.StreamPart{
			Type: provider.StreamPartTypeFinish,
			Finish: &provider.Finish{
				Reason: mapFinishReason(toString(payload["finishReason"])),
				Usage:  usage,
			},
		}
	case "raw":
		if !includeRaw {
			return provider.StreamPart{}, false, nil
		}
		part = provider.StreamPart{Type: provider.StreamPartTypeRaw, Raw: payload["rawValue"]}
	case "error":
		part = provider.StreamPart{
			Type:  provider.StreamPartTypeError,
			Error: &provider.StreamError{Err: fmt.Errorf("%v", payload["error"])},
		}
	default:
		return provider.StreamPart{}, false, fmt.Errorf("unsupported stream part type %q", partType)
	}

	applyGatewayStreamMetadata(&part, payload)
	return part, true, nil
}

func applyGatewayStreamMetadata(part *provider.StreamPart, payload map[string]any) {
	if part == nil {
		return
	}
	if warnings := parseWarnings(payload["warnings"]); len(warnings) > 0 {
		part.Warnings = warnings
	}
	if metadata := parseProviderMetadata(payload["providerMetadata"]); metadata != nil {
		part.ProviderMetadata = metadata
		if part.ResponseMetadata != nil {
			part.ResponseMetadata.ProviderMetadata = metadata
		}
	}
}

func parseUsage(raw any) *provider.LanguageModelUsage {
	mapValue, ok := raw.(map[string]any)
	if !ok {
		return nil
	}
	usage := &provider.LanguageModelUsage{
		PromptTokens:     toInt(mapValue["prompt_tokens"]),
		CompletionTokens: toInt(mapValue["completion_tokens"]),
		TotalTokens:      toInt(mapValue["total_tokens"]),
	}
	if usage.TotalTokens == 0 {
		usage.TotalTokens = usage.PromptTokens + usage.CompletionTokens
	}
	return usage
}

func parseWarnings(raw any) []provider.Warning {
	items, ok := raw.([]any)
	if !ok {
		return nil
	}
	warnings := make([]provider.Warning, 0, len(items))
	for _, item := range items {
		warningMap, ok := item.(map[string]any)
		if !ok {
			continue
		}
		warnings = append(warnings, provider.Warning{
			Category: provider.WarningCategory(toString(coalesceString(warningMap["category"], warningMap["type"]))),
			Message:  toString(warningMap["message"]),
			Metadata: parseMetadataMap(warningMap["metadata"]),
		})
	}
	return warnings
}

func parseProviderMetadata(raw any) provider.ProviderMetadata {
	mapValue, ok := raw.(map[string]any)
	if !ok {
		return nil
	}
	metadata := provider.ProviderMetadata{}
	for key, value := range mapValue {
		inner, ok := value.(map[string]any)
		if !ok {
			continue
		}
		metadata[key] = inner
	}
	if len(metadata) == 0 {
		return nil
	}
	return metadata
}

func parseMetadataMap(raw any) map[string]any {
	metadata, ok := raw.(map[string]any)
	if !ok {
		return nil
	}
	return metadata
}

func parseHeaderMap(raw any) map[string][]string {
	mapValue, ok := raw.(map[string]any)
	if !ok {
		return nil
	}
	headers := map[string][]string{}
	for key, value := range mapValue {
		switch typed := value.(type) {
		case []any:
			var values []string
			for _, item := range typed {
				values = append(values, toString(item))
			}
			headers[key] = values
		case []string:
			headers[key] = typed
		case string:
			headers[key] = []string{typed}
		}
	}
	if len(headers) == 0 {
		return nil
	}
	return headers
}

func mapGatewayResponseError(response *http.Response, body []byte, providerID provider.ProviderID, modelID provider.ModelID) error {
	parsed := decodeGatewayErrorBody(body)
	err := CreateGatewayErrorFromResponse(GatewayErrorResponseOptions{
		Response:       parsed,
		StatusCode:     response.StatusCode,
		DefaultMessage: "Gateway request failed",
		AuthMethod:     GatewayAuthMethodAPIKey,
	})
	metadata := gatewayErrorMetadataFromResponse(response, body, providerID, modelID)
	return MapGatewayErrorToProvider(err, metadata)
}

func mapGatewayRequestError(err error, providerID provider.ProviderID, modelID provider.ModelID, response *http.Response) error {
	wrapped := CreateGatewayErrorFromResponse(GatewayErrorResponseOptions{
		Response:       map[string]any{},
		StatusCode:     http.StatusInternalServerError,
		DefaultMessage: fmt.Sprintf("Gateway request failed: %s", err.Error()),
		Cause:          err,
		AuthMethod:     GatewayAuthMethodAPIKey,
	})
	var body []byte
	if response != nil {
		if response.Body != nil {
			body, _ = readGatewayResponseBody(response)
		}
	}
	metadata := gatewayErrorMetadataFromResponse(response, body, providerID, modelID)
	return MapGatewayErrorToProvider(wrapped, metadata)
}

func readGatewayResponseBody(response *http.Response) ([]byte, error) {
	if response == nil || response.Body == nil {
		return nil, errors.New("missing response body")
	}
	body, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, err
	}
	return body, nil
}

func decodeGatewayErrorBody(body []byte) any {
	trimmed := strings.TrimSpace(string(body))
	if trimmed == "" {
		return map[string]any{}
	}
	var parsed any
	if json.Unmarshal([]byte(trimmed), &parsed) == nil {
		return parsed
	}
	return trimmed
}

func gatewayErrorMetadataFromResponse(response *http.Response, body []byte, providerID provider.ProviderID, modelID provider.ModelID) GatewayErrorMetadata {
	metadata := GatewayErrorMetadata{
		ProviderID: providerID,
		ModelID:    modelID,
	}
	if response != nil {
		metadata.ResponseHeaders = response.Header.Clone()
		metadata.RequestID = response.Header.Get("X-Request-Id")
		if metadata.RequestID == "" {
			metadata.RequestID = response.Header.Get("X-Request-ID")
		}
	}
	if len(body) > 0 {
		metadata.ResponseBody = string(body)
	}
	return metadata
}

func mapFinishReason(reason string) provider.FinishReason {
	switch strings.ToLower(reason) {
	case "stop":
		return provider.FinishReasonStop
	case "length":
		return provider.FinishReasonLength
	case "tool-calls":
		return provider.FinishReasonToolCalls
	case "content-filter":
		return provider.FinishReasonContentFilter
	case "error":
		return provider.FinishReasonError
	default:
		if reason == "" {
			return ""
		}
		return provider.FinishReasonOther
	}
}

func decodeToolCall(raw any) (provider.ToolCall, error) {
	if raw == nil {
		return provider.ToolCall{}, nil
	}
	bytesValue, err := json.Marshal(raw)
	if err != nil {
		return provider.ToolCall{}, err
	}
	var call provider.ToolCall
	if err := json.Unmarshal(bytesValue, &call); err != nil {
		return provider.ToolCall{}, err
	}
	return call, nil
}

func decodeToolResult(raw any) (provider.ToolResult, error) {
	if raw == nil {
		return provider.ToolResult{}, nil
	}
	bytesValue, err := json.Marshal(raw)
	if err != nil {
		return provider.ToolResult{}, err
	}
	var result provider.ToolResult
	if err := json.Unmarshal(bytesValue, &result); err != nil {
		return provider.ToolResult{}, err
	}
	return result, nil
}

func decodeSource(raw any) (provider.Source, error) {
	if raw == nil {
		return provider.Source{}, nil
	}
	bytesValue, err := json.Marshal(raw)
	if err != nil {
		return provider.Source{}, err
	}
	var source provider.Source
	if err := json.Unmarshal(bytesValue, &source); err != nil {
		return provider.Source{}, err
	}
	return source, nil
}

func toString(value any) string {
	if value == nil {
		return ""
	}
	if typed, ok := value.(string); ok {
		return typed
	}
	return fmt.Sprintf("%v", value)
}

func coalesceString(primary any, fallback any) string {
	value := toString(primary)
	if value != "" {
		return value
	}
	return toString(fallback)
}

func toInt(value any) int {
	switch typed := value.(type) {
	case int:
		return typed
	case int64:
		return int(typed)
	case float64:
		return int(typed)
	case float32:
		return int(typed)
	case json.Number:
		if parsed, err := typed.Int64(); err == nil {
			return int(parsed)
		}
	}
	return 0
}
