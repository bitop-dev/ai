package ai

import (
	"context"
	"time"

	"github.com/bitop-dev/ai/pkg/provider"
)

const (
	TelemetryOperationGenerateText   = "GenerateText"
	TelemetryOperationStreamText     = "StreamText"
	TelemetryOperationGenerateObject = "GenerateObject"
	TelemetryOperationStreamObject   = "StreamObject"
)

type TelemetryRequest struct {
	Operation string
	Provider  provider.ProviderID
	Model     provider.ModelID
	Metadata  map[string]any
}

type TelemetrySpanEnd struct {
	Duration         time.Duration
	Usage            *provider.LanguageModelUsage
	Warnings         []provider.Warning
	ResponseMetadata *provider.ResponseMetadata
	ProviderMetadata provider.ProviderMetadata
}

type TelemetrySpanError struct {
	Duration time.Duration
	Err      error
	Warnings []provider.Warning
}

type TelemetrySpan interface {
	End(ctx context.Context, info TelemetrySpanEnd)
	Error(ctx context.Context, info TelemetrySpanError)
}

type Telemetry interface {
	Start(ctx context.Context, info TelemetryRequest) TelemetrySpan
}

type NoopTelemetry struct{}

func (NoopTelemetry) Start(ctx context.Context, info TelemetryRequest) TelemetrySpan {
	return NoopSpan{}
}

type NoopSpan struct{}

func (NoopSpan) End(ctx context.Context, info TelemetrySpanEnd)     {}
func (NoopSpan) Error(ctx context.Context, info TelemetrySpanError) {}
