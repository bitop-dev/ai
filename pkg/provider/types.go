package provider

// ContentPartType identifies the variant of content part.
type ContentPartType string

const (
	ContentPartTypeText       ContentPartType = "text"
	ContentPartTypeToolCall   ContentPartType = "tool-call"
	ContentPartTypeToolResult ContentPartType = "tool-result"
	ContentPartTypeSource     ContentPartType = "source"
	ContentPartTypeReasoning  ContentPartType = "reasoning"
	ContentPartTypeImage      ContentPartType = "image"
	ContentPartTypeFile       ContentPartType = "file"
)

// MessageRole identifies the role a message was authored as.
type MessageRole string

const (
	RoleSystem    MessageRole = "system"
	RoleUser      MessageRole = "user"
	RoleAssistant MessageRole = "assistant"
	RoleTool      MessageRole = "tool"
)

// Prompt represents a model prompt with ordered messages.
type Prompt struct {
	Messages []ModelMessage
}

// ModelMessage represents a single chat-style message.
type ModelMessage struct {
	Role       MessageRole
	Content    []ContentPart
	Name       string
	ToolCallID string
}

// ContentPart represents a single piece of content within a message.
type ContentPart interface {
	ContentType() ContentPartType
}

// TextContent is plain text content.
type TextContent struct {
	Text string
}

func (TextContent) ContentType() ContentPartType { return ContentPartTypeText }

// ToolCall represents a tool invocation request.
type ToolCall struct {
	ID        string
	Name      string
	Arguments map[string]any
}

// ToolCallContent wraps a tool call as message content.
type ToolCallContent struct {
	ToolCall ToolCall
}

func (ToolCallContent) ContentType() ContentPartType { return ContentPartTypeToolCall }

// ToolResult represents the result from invoking a tool.
type ToolResult struct {
	ID      string
	Name    string
	Result  any
	IsError bool
}

// ToolResultContent wraps a tool result as message content.
type ToolResultContent struct {
	ToolResult ToolResult
}

func (ToolResultContent) ContentType() ContentPartType { return ContentPartTypeToolResult }

// Source represents a citation or source reference.
type Source struct {
	ID    string
	URL   string
	Title string
}

// SourceContent wraps a source reference as message content.
type SourceContent struct {
	Source Source
}

func (SourceContent) ContentType() ContentPartType { return ContentPartTypeSource }

// ReasoningContent captures model reasoning text.
type ReasoningContent struct {
	Text string
}

func (ReasoningContent) ContentType() ContentPartType { return ContentPartTypeReasoning }

// ImageContent captures an image reference or payload.
type ImageContent struct {
	URL       string
	MediaType string
	Data      []byte
	AltText   string
}

func (ImageContent) ContentType() ContentPartType { return ContentPartTypeImage }

// FileContent captures a file reference or payload.
type FileContent struct {
	Name      string
	URL       string
	MediaType string
	Data      []byte
}

func (FileContent) ContentType() ContentPartType { return ContentPartTypeFile }
