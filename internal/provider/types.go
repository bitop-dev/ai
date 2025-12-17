package provider

import "encoding/json"

type FinishReason string

type Usage struct {
	PromptTokens     int
	CompletionTokens int
	TotalTokens      int
}

type Role string

const (
	RoleSystem    Role = "system"
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
	RoleTool      Role = "tool"
)

type Message struct {
	Role    Role
	Content []ContentPart
	Name    string

	// ToolCallID is used for tool result messages (role=tool) to associate the
	// result with a prior tool call.
	ToolCallID string
}

type ContentPart interface {
	isContentPart()
}

type TextPart struct{ Text string }

func (TextPart) isContentPart() {}

type ToolCallPart struct {
	ID   string
	Name string
	Args json.RawMessage
}

func (ToolCallPart) isContentPart() {}

type ToolDefinition struct {
	Name        string
	Description string
	InputSchema json.RawMessage
}

type Delta struct {
	Text      string
	ToolCalls []ToolCallDelta
}

type ToolCallDelta struct {
	Index int
	ID    string
	Name  string
	// ArgumentsDelta contains a fragment of the JSON arguments string as it
	// arrives during streaming (it is not guaranteed to be valid JSON by itself).
	ArgumentsDelta string
}
