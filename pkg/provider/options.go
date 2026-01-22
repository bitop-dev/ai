package provider

import "time"

// ProviderOptions represents provider-specific overrides keyed by provider ID.
type ProviderOptions map[string]JSONObject

// RequestOptions captures per-request overrides for providers.
type RequestOptions struct {
	Headers         map[string]string
	Timeout         time.Duration
	IdempotencyKey  string
	Metadata        map[string]any
	ProviderOptions ProviderOptions
}

// ResponseFormatType identifies the response format type.
type ResponseFormatType string

const (
	ResponseFormatTypeText ResponseFormatType = "text"
	ResponseFormatTypeJSON ResponseFormatType = "json"
)

// ResponseFormat specifies the desired output format.
type ResponseFormat struct {
	Type        ResponseFormatType
	Schema      JSONSchema
	Name        string
	Description string
}

// ToolChoiceType identifies how tools should be selected.
type ToolChoiceType string

const (
	ToolChoiceTypeAuto     ToolChoiceType = "auto"
	ToolChoiceTypeNone     ToolChoiceType = "none"
	ToolChoiceTypeRequired ToolChoiceType = "required"
	ToolChoiceTypeTool     ToolChoiceType = "tool"
)

// ToolChoice describes tool selection behavior.
type ToolChoice struct {
	Type     ToolChoiceType
	ToolName string
}
