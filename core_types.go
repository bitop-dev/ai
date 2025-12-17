package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"time"
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

	Headers    map[string]string
	MaxRetries *int
	Timeout    time.Duration

	// OnToolProgress is called when a tool reports progress during execution.
	// Tools created via NewTool/NewDynamicTool can report progress via ToolExecutionMeta.Report.
	OnToolProgress func(event ToolProgressEvent)

	// OnStepFinish is called when a model generation step is complete (including
	// any tool calls and tool results produced by that step).
	OnStepFinish func(event StepFinishEvent)

	// PrepareStep is called before each model generation step in a multi-step tool loop.
	// It can override the messages and active tools for that step.
	PrepareStep func(event PrepareStepEvent) (PrepareStepResult, error)

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

	Steps    []Step
	Response Response
}

type StreamTextRequest = GenerateTextRequest

type TextStream struct {
	next    func() bool
	delta   func() string
	message func() *Message
	usage   func() Usage
	finish  func() FinishReason
	steps   func() []Step
	resp    func() Response
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

func (s *TextStream) Steps() []Step {
	if s == nil || s.steps == nil {
		return nil
	}
	return s.steps()
}

func (s *TextStream) Response() Response {
	if s == nil || s.resp == nil {
		return Response{}
	}
	return s.resp()
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
	steps func() []Step,
	resp func() Response,
	err func() error,
	close func() error,
) *TextStream {
	return &TextStream{
		next:    next,
		delta:   delta,
		message: message,
		usage:   usage,
		finish:  finish,
		steps:   steps,
		resp:    resp,
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

	// Tool input lifecycle hooks (streaming only).
	// These are called only for StreamText (GenerateText does not stream tool inputs).
	OnInputStart     func(event ToolInputStartEvent)
	OnInputDelta     func(event ToolInputDeltaEvent)
	OnInputAvailable func(event ToolInputAvailableEvent)
}

type ToolHandler func(ctx context.Context, input json.RawMessage) (any, error)

type ToolInputStartEvent struct {
	ToolName      string
	ToolCallID    string
	ToolCallIndex int
}

type ToolInputDeltaEvent struct {
	ToolName      string
	ToolCallID    string
	ToolCallIndex int

	InputTextDelta string
}

type ToolInputAvailableEvent struct {
	ToolName      string
	ToolCallID    string
	ToolCallIndex int

	Input json.RawMessage
}

type ToolProgressEvent struct {
	ToolName      string
	ToolCallID    string
	ToolCallIndex int

	Data any
}

type Step struct {
	StepNumber int

	Text        string
	Message     Message
	ToolCalls   []ToolCallPart
	ToolResults []Message

	FinishReason FinishReason
	Usage        Usage

	ActiveTools []string
}

type StepFinishEvent struct {
	Step Step
}

type PrepareStepEvent struct {
	StepNumber int
	Steps      []Step

	Messages []Message
}

type PrepareStepResult struct {
	// Model overrides the model for this step. The provider must match the original request.
	Model ModelRef

	// Messages overrides the messages used for this step (and becomes the base
	// for following steps).
	Messages []Message

	// ActiveTools restricts tools available to the model for this step.
	// When empty/nil, all tools are active.
	ActiveTools []string
}

type StopConditionEvent struct {
	Steps []Step
}

type StopCondition func(event StopConditionEvent) bool

func StepCountIs(maxSteps int) StopCondition {
	return func(event StopConditionEvent) bool {
		return len(event.Steps) >= maxSteps
	}
}

func HasToolCall(toolName string) StopCondition {
	return func(event StopConditionEvent) bool {
		for _, s := range event.Steps {
			for _, tc := range s.ToolCalls {
				if tc.Name == toolName {
					return true
				}
			}
		}
		return false
	}
}

type Response struct {
	Messages []Message
}

type ToolLoopOptions struct {
	MaxIterations int

	// StopWhen determines when to stop the internal tool loop. It is only
	// evaluated when the last step contains tool results.
	StopWhen StopCondition
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

// ImagePart represents a multimodal image input in chat messages.
// Provide either URL (remote or data URL) or Bytes/Base64.
type ImagePart struct {
	URL       string
	MediaType string
	Bytes     []byte
	Base64    string
}

func (ImagePart) isContentPart() {}

// AudioPart represents a multimodal audio input in chat messages.
// Provide Bytes/Base64 plus a Format (e.g. "wav" or "mp3") when required.
type AudioPart struct {
	Format string
	Bytes  []byte
	Base64 string
}

func (AudioPart) isContentPart() {}

func ImageURL(url string) ImagePart { return ImagePart{URL: url} }

func ImageBytes(mediaType string, b []byte) ImagePart {
	return ImagePart{MediaType: mediaType, Bytes: b}
}

func ImageBase64(mediaType string, b64 string) ImagePart {
	return ImagePart{MediaType: mediaType, Base64: b64}
}

func AudioBytes(format string, b []byte) AudioPart { return AudioPart{Format: format, Bytes: b} }

func AudioBase64(format string, b64 string) AudioPart { return AudioPart{Format: format, Base64: b64} }

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
