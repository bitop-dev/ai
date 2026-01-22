package ai

import (
	"context"
	"reflect"
	"testing"
	"time"

	"github.com/vercel/ai-sdk-go/pkg/provider"
)

type stubLanguageModel struct {
	parts       []provider.StreamPart
	callOptions provider.LanguageModelV3CallOptions
	ctx         context.Context
	request     *provider.LanguageModelV3Request
	response    *provider.LanguageModelV3Response
}

func (stubLanguageModel) SpecificationVersion() provider.SpecificationVersion {
	return provider.SpecificationVersionV3
}

func (stubLanguageModel) ProviderID() provider.ProviderID { return provider.ProviderID("stub") }

func (stubLanguageModel) ModelID() provider.ModelID { return provider.ModelID("test-model") }

func (stubLanguageModel) SupportedURLs() provider.SupportedURLPatterns { return nil }

func (m *stubLanguageModel) DoGenerate(ctx context.Context, options provider.LanguageModelV3CallOptions) (provider.LanguageModelV3GenerateResult, error) {
	m.ctx = ctx
	m.callOptions = options
	return provider.LanguageModelV3GenerateResult{}, nil
}

func (m *stubLanguageModel) DoStream(ctx context.Context, options provider.LanguageModelV3CallOptions) (provider.LanguageModelV3StreamResult, error) {
	m.ctx = ctx
	m.callOptions = options
	stream := make(chan provider.StreamPart, len(m.parts))
	for _, part := range m.parts {
		stream <- part
	}
	close(stream)
	return provider.LanguageModelV3StreamResult{
		Stream:   stream,
		Request:  m.request,
		Response: m.response,
	}, nil
}

func TestStreamTextOptionsAndCancel(t *testing.T) {
	maxTokens := 42
	temperature := 0.7
	topP := 0.9
	topK := 4
	presence := 0.1
	frequency := 0.2
	seed := 11
	includeRaw := true
	requestOptions := provider.RequestOptions{
		Headers:        map[string]string{"X-Test": "true"},
		Timeout:        time.Second,
		IdempotencyKey: "idem",
		Metadata:       map[string]any{"trace": "1"},
		ProviderOptions: provider.ProviderOptions{
			"stub": {"mode": "fast"},
		},
	}

	model := &stubLanguageModel{parts: []provider.StreamPart{
		{Type: provider.StreamPartTypeTextStart, TextStart: &provider.TextStart{Text: "hello"}},
		{Type: provider.StreamPartTypeFinish, Finish: &provider.Finish{Reason: provider.FinishReasonStop}},
	}}

	result, err := StreamText(context.Background(), model, StreamTextOptions{
		Prompt: provider.Prompt{Messages: []provider.ModelMessage{{
			Role:    provider.RoleUser,
			Content: []provider.ContentPart{provider.TextContent{Text: "hi"}},
		}}},
		MaxOutputTokens:  &maxTokens,
		Temperature:      &temperature,
		StopSequences:    []string{"\n"},
		TopP:             &topP,
		TopK:             &topK,
		PresencePenalty:  &presence,
		FrequencyPenalty: &frequency,
		ResponseFormat:   &provider.ResponseFormat{Type: provider.ResponseFormatTypeText},
		Seed:             &seed,
		ToolChoice:       &provider.ToolChoice{Type: provider.ToolChoiceTypeAuto},
		IncludeRawChunks: &includeRaw,
		RequestOptions:   requestOptions,
	})
	if err != nil {
		t.Fatalf("StreamText returned error: %v", err)
	}

	var gotTypes []provider.StreamPartType
	for result.Stream.Next() {
		gotTypes = append(gotTypes, result.Stream.Value().Type)
	}
	if err := result.Stream.Err(); err != nil {
		t.Fatalf("StreamText stream error: %v", err)
	}
	result.Stream.Close()

	select {
	case <-model.ctx.Done():
	default:
		t.Fatalf("expected stream cancel to cancel context")
	}

	if model.callOptions.MaxOutputTokens != maxTokens {
		t.Fatalf("MaxOutputTokens mismatch: got %d want %d", model.callOptions.MaxOutputTokens, maxTokens)
	}
	if model.callOptions.Temperature != temperature {
		t.Fatalf("Temperature mismatch: got %v want %v", model.callOptions.Temperature, temperature)
	}
	if model.callOptions.TopP != topP {
		t.Fatalf("TopP mismatch: got %v want %v", model.callOptions.TopP, topP)
	}
	if model.callOptions.TopK != topK {
		t.Fatalf("TopK mismatch: got %d want %d", model.callOptions.TopK, topK)
	}
	if model.callOptions.PresencePenalty != presence {
		t.Fatalf("PresencePenalty mismatch: got %v want %v", model.callOptions.PresencePenalty, presence)
	}
	if model.callOptions.FrequencyPenalty != frequency {
		t.Fatalf("FrequencyPenalty mismatch: got %v want %v", model.callOptions.FrequencyPenalty, frequency)
	}
	if model.callOptions.Seed != seed {
		t.Fatalf("Seed mismatch: got %d want %d", model.callOptions.Seed, seed)
	}
	if model.callOptions.IncludeRawChunks != includeRaw {
		t.Fatalf("IncludeRawChunks mismatch: got %v want %v", model.callOptions.IncludeRawChunks, includeRaw)
	}
	if !reflect.DeepEqual(model.callOptions.RequestOptions, requestOptions) {
		t.Fatalf("RequestOptions mismatch: got %#v want %#v", model.callOptions.RequestOptions, requestOptions)
	}
	if !reflect.DeepEqual(model.callOptions.ProviderOptions, requestOptions.ProviderOptions) {
		t.Fatalf("ProviderOptions mismatch: got %#v want %#v", model.callOptions.ProviderOptions, requestOptions.ProviderOptions)
	}

	wantTypes := []provider.StreamPartType{provider.StreamPartTypeTextStart, provider.StreamPartTypeFinish}
	if !reflect.DeepEqual(gotTypes, wantTypes) {
		t.Fatalf("stream parts mismatch: got %#v want %#v", gotTypes, wantTypes)
	}
}

func TestGenerateTextAggregates(t *testing.T) {
	usage := &provider.LanguageModelUsage{PromptTokens: 1, CompletionTokens: 2, TotalTokens: 3}
	responseMetadata := &provider.ResponseMetadata{
		RequestID: "req-1",
		Headers:   map[string][]string{"X-Response": {"ok"}},
		ProviderMetadata: provider.ProviderMetadata{
			"stub": {"b": "2"},
		},
	}

	model := &stubLanguageModel{
		parts: []provider.StreamPart{
			{Type: provider.StreamPartTypeTextStart, TextStart: &provider.TextStart{Text: "hello"}},
			{Type: provider.StreamPartTypeTextDelta, TextDelta: &provider.TextDelta{Delta: " world"}, Warnings: []provider.Warning{{Category: provider.WarningCategoryOther, Message: "note"}}},
			{Type: provider.StreamPartTypeResponseMetadata, ResponseMetadata: responseMetadata, ProviderMetadata: provider.ProviderMetadata{"stub": {"a": "1"}}},
			{Type: provider.StreamPartTypeFinish, Finish: &provider.Finish{Reason: provider.FinishReasonStop, Usage: usage}},
		},
		request:  &provider.LanguageModelV3Request{Body: map[string]any{"prompt": "hi"}},
		response: &provider.LanguageModelV3Response{Headers: map[string][]string{"X-Response": {"ok"}}},
	}

	result, err := GenerateText(context.Background(), model, GenerateTextOptions{Prompt: provider.Prompt{Messages: []provider.ModelMessage{{
		Role:    provider.RoleUser,
		Content: []provider.ContentPart{provider.TextContent{Text: "hi"}},
	}}}})
	if err != nil {
		t.Fatalf("GenerateText returned error: %v", err)
	}

	if result.Text != "hello world" {
		t.Fatalf("Text mismatch: got %q want %q", result.Text, "hello world")
	}
	if result.FinishReason != provider.FinishReasonStop {
		t.Fatalf("FinishReason mismatch: got %q want %q", result.FinishReason, provider.FinishReasonStop)
	}
	if !reflect.DeepEqual(result.Usage, usage) {
		t.Fatalf("Usage mismatch: got %#v want %#v", result.Usage, usage)
	}
	if result.ResponseMetadata != responseMetadata {
		t.Fatalf("ResponseMetadata mismatch: got %#v want %#v", result.ResponseMetadata, responseMetadata)
	}
	if len(result.Warnings) != 1 {
		t.Fatalf("Warnings length mismatch: got %d want %d", len(result.Warnings), 1)
	}
	if len(result.Parts) != 4 {
		t.Fatalf("Parts length mismatch: got %d want %d", len(result.Parts), 4)
	}
	if !reflect.DeepEqual(result.Request, model.request) {
		t.Fatalf("Request mismatch: got %#v want %#v", result.Request, model.request)
	}
	if !reflect.DeepEqual(result.Response, model.response) {
		t.Fatalf("Response mismatch: got %#v want %#v", result.Response, model.response)
	}

	metadata := provider.ProviderMetadata{"stub": {"a": "1", "b": "2"}}
	if !reflect.DeepEqual(result.ProviderMetadata, metadata) {
		t.Fatalf("ProviderMetadata mismatch: got %#v want %#v", result.ProviderMetadata, metadata)
	}
}
