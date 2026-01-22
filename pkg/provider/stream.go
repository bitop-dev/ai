package provider

type StreamPartType string

const (
	StreamPartTypeStreamStart      StreamPartType = "stream-start"
	StreamPartTypeTextStart        StreamPartType = "text-start"
	StreamPartTypeTextDelta        StreamPartType = "text-delta"
	StreamPartTypeTextEnd          StreamPartType = "text-end"
	StreamPartTypeToolInputStart   StreamPartType = "tool-input-start"
	StreamPartTypeToolInputDelta   StreamPartType = "tool-input-delta"
	StreamPartTypeToolInputEnd     StreamPartType = "tool-input-end"
	StreamPartTypeToolCall         StreamPartType = "tool-call"
	StreamPartTypeToolResult       StreamPartType = "tool-result"
	StreamPartTypeSource           StreamPartType = "source"
	StreamPartTypeReasoningStart   StreamPartType = "reasoning-start"
	StreamPartTypeReasoningDelta   StreamPartType = "reasoning-delta"
	StreamPartTypeReasoningEnd     StreamPartType = "reasoning-end"
	StreamPartTypeResponseMetadata StreamPartType = "response-metadata"
	StreamPartTypeFinish           StreamPartType = "finish"
	StreamPartTypeError            StreamPartType = "error"
)

type ProviderMetadata map[string]map[string]any

type WarningCategory string

const (
	WarningCategoryUnsupportedOption WarningCategory = "unsupported-option"
	WarningCategoryDeprecatedOption  WarningCategory = "deprecated-option"
	WarningCategoryUnsafeContent     WarningCategory = "unsafe-content"
	WarningCategoryTruncatedResponse WarningCategory = "truncated-response"
	WarningCategoryOther             WarningCategory = "other"
)

type Warning struct {
	Category WarningCategory
	Message  string
	Metadata map[string]any
}

type LanguageModelUsage struct {
	PromptTokens     int
	CompletionTokens int
	TotalTokens      int
}

type EmbeddingUsage struct {
	InputTokens int
	TotalTokens int
}

type ImageUsage struct {
	ImagesGenerated int
}

type FinishReason string

const (
	FinishReasonStop          FinishReason = "stop"
	FinishReasonLength        FinishReason = "length"
	FinishReasonToolCalls     FinishReason = "tool-calls"
	FinishReasonContentFilter FinishReason = "content-filter"
	FinishReasonError         FinishReason = "error"
	FinishReasonOther         FinishReason = "other"
)

type StreamPart struct {
	Type             StreamPartType
	StreamStart      *StreamStart
	TextStart        *TextStart
	TextDelta        *TextDelta
	TextEnd          *TextEnd
	ToolInputStart   *ToolInputStart
	ToolInputDelta   *ToolInputDelta
	ToolInputEnd     *ToolInputEnd
	ToolCall         *ToolCall
	ToolResult       *ToolResult
	Source           *Source
	ReasoningStart   *ReasoningStart
	ReasoningDelta   *ReasoningDelta
	ReasoningEnd     *ReasoningEnd
	ResponseMetadata *ResponseMetadata
	Finish           *Finish
	Error            *StreamError
	ProviderMetadata ProviderMetadata
	Warnings         []Warning
	Raw              any
}

type StreamStart struct {
	ProviderID ProviderID
	ModelID    ModelID
}

type TextStart struct {
	Text string
}

type TextDelta struct {
	Delta string
}

type TextEnd struct {
	Text string
}

type ToolInputStart struct {
	ToolCallID string
	Name       string
}

type ToolInputDelta struct {
	ToolCallID string
	Delta      string
}

type ToolInputEnd struct {
	ToolCallID string
}

type ReasoningStart struct {
	Text string
}

type ReasoningDelta struct {
	Delta string
}

type ReasoningEnd struct {
	Text string
}

type ResponseMetadata struct {
	RequestID        string
	HTTPStatus       int
	Headers          map[string][]string
	ProviderMetadata ProviderMetadata
}

type Finish struct {
	Reason FinishReason
	Usage  *LanguageModelUsage
}

type StreamError struct {
	Err error
}
