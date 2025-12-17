package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
)

type ModelRef interface {
	Provider() string
	Name() string
}

type BaseRequest struct {
	Model ModelRef

	Messages []Message
	Tools    []Tool
	ToolLoop *ToolLoopOptions

	MaxTokens   *int
	Temperature *float32
	TopP        *float32
	Stop        []string

	Metadata map[string]string
}

type GenerateTextRequest struct {
	BaseRequest
}

type GenerateTextResponse struct {
	Text string

	Message      Message
	Usage        Usage
	FinishReason FinishReason
}

type StreamTextRequest = GenerateTextRequest

type TextStream struct {
	next    func() bool
	delta   func() string
	message func() *Message
	usage   func() Usage
	finish  func() FinishReason
	err     func() error
	close   func() error
}

func (s *TextStream) Next() bool {
	if s == nil || s.next == nil {
		return false
	}
	return s.next()
}

func (s *TextStream) Delta() string {
	if s == nil || s.delta == nil {
		return ""
	}
	return s.delta()
}

func (s *TextStream) Message() *Message {
	if s == nil || s.message == nil {
		return nil
	}
	return s.message()
}

func (s *TextStream) Usage() Usage {
	if s == nil || s.usage == nil {
		return Usage{}
	}
	return s.usage()
}

func (s *TextStream) FinishReason() FinishReason {
	if s == nil || s.finish == nil {
		return FinishUnknown
	}
	return s.finish()
}

func (s *TextStream) Err() error {
	if s == nil || s.err == nil {
		return nil
	}
	return s.err()
}

func (s *TextStream) Close() error {
	if s == nil || s.close == nil {
		return nil
	}
	return s.close()
}

func newTextStream(
	next func() bool,
	delta func() string,
	message func() *Message,
	usage func() Usage,
	finish func() FinishReason,
	err func() error,
	close func() error,
) *TextStream {
	return &TextStream{
		next:    next,
		delta:   delta,
		message: message,
		usage:   usage,
		finish:  finish,
		err:     err,
		close:   close,
	}
}

// Iter returns a channel of text deltas. The caller should check Err() after
// the channel is closed and call Close() when done.
//
// Do not call Next() concurrently with Iter().
func (s *TextStream) Iter() <-chan string {
	ch := make(chan string)
	go func() {
		defer close(ch)
		for s.Next() {
			ch <- s.Delta()
		}
	}()
	return ch
}

// Reader exposes the stream as an io.Reader of text deltas.
//
// Do not call Next() concurrently with Reader().
func (s *TextStream) Reader() io.Reader {
	return &textStreamReader{stream: s}
}

type textStreamReader struct {
	stream *TextStream
	buf    []byte
	done   bool
}

func (r *textStreamReader) Read(p []byte) (int, error) {
	if r.done {
		return 0, io.EOF
	}
	for len(r.buf) == 0 {
		if r.stream.Next() {
			r.buf = []byte(r.stream.Delta())
			continue
		}
		if err := r.stream.Err(); err != nil {
			r.done = true
			return 0, err
		}
		r.done = true
		return 0, io.EOF
	}

	n := copy(p, r.buf)
	r.buf = r.buf[n:]
	return n, nil
}

type GenerateObjectRequest[T any] struct {
	BaseRequest

	Schema  Schema
	Example *T

	Strict     *bool
	MaxRetries *int
}

type GenerateObjectResponse[T any] struct {
	Object T

	RawJSON         json.RawMessage
	Message         Message
	Usage           Usage
	FinishReason    FinishReason
	ValidationError error
}

type StreamObjectRequest[T any] = GenerateObjectRequest[T]

type ObjectStream[T any] struct {
	next    func() bool
	raw     func() json.RawMessage
	partial func() map[string]any
	object  func() *T
	err     func() error
	close   func() error
}

func (s *ObjectStream[T]) Next() bool {
	if s == nil || s.next == nil {
		return false
	}
	return s.next()
}

// Raw returns the accumulated tool-call arguments so far (may be invalid JSON
// mid-stream).
func (s *ObjectStream[T]) Raw() json.RawMessage {
	if s == nil || s.raw == nil {
		return nil
	}
	return s.raw()
}

// Partial returns the latest best-effort partial object, or nil if the current
// accumulated JSON is not parseable yet.
func (s *ObjectStream[T]) Partial() map[string]any {
	if s == nil || s.partial == nil {
		return nil
	}
	return s.partial()
}

// Object returns the final decoded object once the stream has completed.
func (s *ObjectStream[T]) Object() *T {
	if s == nil || s.object == nil {
		return nil
	}
	return s.object()
}

func (s *ObjectStream[T]) Err() error {
	if s == nil || s.err == nil {
		return nil
	}
	return s.err()
}

func (s *ObjectStream[T]) Close() error {
	if s == nil || s.close == nil {
		return nil
	}
	return s.close()
}

func newObjectStream[T any](
	next func() bool,
	raw func() json.RawMessage,
	partial func() map[string]any,
	object func() *T,
	err func() error,
	close func() error,
) *ObjectStream[T] {
	return &ObjectStream[T]{
		next:    next,
		raw:     raw,
		partial: partial,
		object:  object,
		err:     err,
		close:   close,
	}
}

type Schema struct {
	JSON json.RawMessage
}

func JSONSchema(raw json.RawMessage) Schema {
	return Schema{JSON: raw}
}

type Tool struct {
	Name        string
	Description string
	InputSchema Schema
	Handler     ToolHandler
}

type ToolHandler func(ctx context.Context, input json.RawMessage) (any, error)

type ToolLoopOptions struct {
	MaxIterations int
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

	ToolCallID string // required for role=tool messages
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

type ImagePart struct{}

func (ImagePart) isContentPart() {}

type AudioPart struct{}

func (AudioPart) isContentPart() {}

func System(text string) Message {
	return Message{Role: RoleSystem, Content: []ContentPart{TextPart{Text: text}}}
}

func User(text string) Message {
	return Message{Role: RoleUser, Content: []ContentPart{TextPart{Text: text}}}
}

func Assistant(text string) Message {
	return Message{Role: RoleAssistant, Content: []ContentPart{TextPart{Text: text}}}
}

func ToolResult(toolName string, value any) Message {
	raw, err := json.Marshal(value)
	if err != nil {
		raw = json.RawMessage(fmt.Sprintf(`{"error":%q}`, err.Error()))
	}
	return Message{
		Role: RoleTool,
		Name: toolName,
		// ToolCallID must be set to associate the result with a model tool call.
		// Prefer ToolResultForCall when constructing tool results.
		Content: []ContentPart{TextPart{Text: string(raw)}},
	}
}

func ToolResultForCall(toolCallID, toolName string, value any) Message {
	m := ToolResult(toolName, value)
	m.ToolCallID = toolCallID
	return m
}

type FinishReason string

const (
	FinishStop          FinishReason = "stop"
	FinishLength        FinishReason = "length"
	FinishToolCalls     FinishReason = "tool_calls"
	FinishContentFilter FinishReason = "content_filter"
	FinishError         FinishReason = "error"
	FinishUnknown       FinishReason = "unknown"
)

type Usage struct {
	PromptTokens     int
	CompletionTokens int
	TotalTokens      int

	PromptTokensDetails     map[string]int
	CompletionTokensDetails map[string]int
}
