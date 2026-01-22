package ai

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/bitop-dev/ai/pkg/provider"
)

type captureTelemetry struct {
	requests []TelemetryRequest
	spans    []*captureSpan
}

func (c *captureTelemetry) Start(ctx context.Context, info TelemetryRequest) TelemetrySpan {
	span := &captureSpan{}
	c.requests = append(c.requests, info)
	c.spans = append(c.spans, span)
	return span
}

type captureSpan struct {
	end *TelemetrySpanEnd
	err *TelemetrySpanError
}

func (c *captureSpan) End(ctx context.Context, info TelemetrySpanEnd) {
	c.end = &info
}

func (c *captureSpan) Error(ctx context.Context, info TelemetrySpanError) {
	c.err = &info
}

func TestGenerateTextTelemetryEnd(t *testing.T) {
	usage := &provider.LanguageModelUsage{PromptTokens: 1, CompletionTokens: 2, TotalTokens: 3}
	responseMetadata := &provider.ResponseMetadata{
		RequestID: "req-1",
		Headers:   map[string][]string{"X-Response": {"ok"}},
		ProviderMetadata: provider.ProviderMetadata{
			"stub": {"b": "2"},
		},
	}
	telemetry := &captureTelemetry{}
	model := &stubLanguageModel{
		parts: []provider.StreamPart{
			{Type: provider.StreamPartTypeTextStart, TextStart: &provider.TextStart{Text: "hello"}},
			{Type: provider.StreamPartTypeTextDelta, TextDelta: &provider.TextDelta{Delta: " world"}, Warnings: []provider.Warning{{Category: provider.WarningCategoryOther, Message: "note"}}},
			{Type: provider.StreamPartTypeResponseMetadata, ResponseMetadata: responseMetadata, ProviderMetadata: provider.ProviderMetadata{"stub": {"a": "1"}}},
			{Type: provider.StreamPartTypeFinish, Finish: &provider.Finish{Reason: provider.FinishReasonStop, Usage: usage}},
		},
	}

	_, err := GenerateText(context.Background(), model, GenerateTextOptions{Telemetry: telemetry, Prompt: provider.Prompt{Messages: []provider.ModelMessage{{
		Role:    provider.RoleUser,
		Content: []provider.ContentPart{provider.TextContent{Text: "hi"}},
	}}}})
	if err != nil {
		t.Fatalf("GenerateText returned error: %v", err)
	}

	if len(telemetry.spans) != 1 {
		t.Fatalf("expected telemetry span, got %d", len(telemetry.spans))
	}
	span := telemetry.spans[0]
	if span.err != nil {
		t.Fatalf("unexpected telemetry error: %#v", span.err)
	}
	if span.end == nil {
		t.Fatalf("expected telemetry end")
	}
	if span.end.Duration <= 0 {
		t.Fatalf("expected duration to be set")
	}
	if !reflect.DeepEqual(span.end.Usage, usage) {
		t.Fatalf("usage mismatch: got %#v want %#v", span.end.Usage, usage)
	}
	if len(span.end.Warnings) != 1 {
		t.Fatalf("warnings mismatch: got %d want %d", len(span.end.Warnings), 1)
	}
	if span.end.ResponseMetadata != responseMetadata {
		t.Fatalf("response metadata mismatch: got %#v want %#v", span.end.ResponseMetadata, responseMetadata)
	}
	metadata := provider.ProviderMetadata{"stub": {"a": "1", "b": "2"}}
	if !reflect.DeepEqual(span.end.ProviderMetadata, metadata) {
		t.Fatalf("provider metadata mismatch: got %#v want %#v", span.end.ProviderMetadata, metadata)
	}
	if len(telemetry.requests) != 1 {
		t.Fatalf("expected telemetry request, got %d", len(telemetry.requests))
	}
	request := telemetry.requests[0]
	if request.Operation != TelemetryOperationGenerateText {
		t.Fatalf("operation mismatch: got %q want %q", request.Operation, TelemetryOperationGenerateText)
	}
}

type errorStreamModel struct{ stubLanguageModel }

func (m *errorStreamModel) DoStream(ctx context.Context, options provider.LanguageModelV3CallOptions) (provider.LanguageModelV3StreamResult, error) {
	return provider.LanguageModelV3StreamResult{}, errors.New("stream failure")
}

func TestStreamTextTelemetryError(t *testing.T) {
	telemetry := &captureTelemetry{}
	model := &errorStreamModel{}

	_, err := StreamText(context.Background(), model, StreamTextOptions{
		Telemetry: telemetry,
		Prompt: provider.Prompt{Messages: []provider.ModelMessage{{
			Role:    provider.RoleUser,
			Content: []provider.ContentPart{provider.TextContent{Text: "hi"}},
		}}},
	})
	if err == nil {
		t.Fatalf("expected error")
	}

	if len(telemetry.spans) != 1 {
		t.Fatalf("expected telemetry span, got %d", len(telemetry.spans))
	}
	span := telemetry.spans[0]
	if span.end != nil {
		t.Fatalf("expected telemetry error, got end")
	}
	if span.err == nil {
		t.Fatalf("expected telemetry error")
	}
	if span.err.Duration <= 0 {
		t.Fatalf("expected duration to be set")
	}
}
