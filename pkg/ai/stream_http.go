package ai

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/vercel/ai-sdk-go/pkg/provider"
	"github.com/vercel/ai-sdk-go/pkg/providerutils"
)

var ErrNilStream = errors.New("stream is nil")
var ErrNilResponseWriter = errors.New("response writer is nil")

type streamPartPayload struct {
	Type             provider.StreamPartType    `json:"type"`
	StreamStart      *provider.StreamStart      `json:"streamStart,omitempty"`
	TextStart        *provider.TextStart        `json:"textStart,omitempty"`
	TextDelta        *provider.TextDelta        `json:"textDelta,omitempty"`
	TextEnd          *provider.TextEnd          `json:"textEnd,omitempty"`
	ToolInputStart   *provider.ToolInputStart   `json:"toolInputStart,omitempty"`
	ToolInputDelta   *provider.ToolInputDelta   `json:"toolInputDelta,omitempty"`
	ToolInputEnd     *provider.ToolInputEnd     `json:"toolInputEnd,omitempty"`
	ToolCall         *provider.ToolCall         `json:"toolCall,omitempty"`
	ToolResult       *provider.ToolResult       `json:"toolResult,omitempty"`
	Source           *provider.Source           `json:"source,omitempty"`
	ReasoningStart   *provider.ReasoningStart   `json:"reasoningStart,omitempty"`
	ReasoningDelta   *provider.ReasoningDelta   `json:"reasoningDelta,omitempty"`
	ReasoningEnd     *provider.ReasoningEnd     `json:"reasoningEnd,omitempty"`
	ResponseMetadata *provider.ResponseMetadata `json:"responseMetadata,omitempty"`
	Finish           *provider.Finish           `json:"finish,omitempty"`
	Error            *streamErrorPayload        `json:"error,omitempty"`
	ProviderMetadata provider.ProviderMetadata  `json:"providerMetadata,omitempty"`
	Warnings         []provider.Warning         `json:"warnings,omitempty"`
	Raw              any                        `json:"raw,omitempty"`
}

type streamErrorPayload struct {
	Message string `json:"message"`
}

func formatStreamPart(part provider.StreamPart) ([]byte, error) {
	payload := streamPartPayload{
		Type:             part.Type,
		StreamStart:      part.StreamStart,
		TextStart:        part.TextStart,
		TextDelta:        part.TextDelta,
		TextEnd:          part.TextEnd,
		ToolInputStart:   part.ToolInputStart,
		ToolInputDelta:   part.ToolInputDelta,
		ToolInputEnd:     part.ToolInputEnd,
		ToolCall:         part.ToolCall,
		ToolResult:       part.ToolResult,
		Source:           part.Source,
		ReasoningStart:   part.ReasoningStart,
		ReasoningDelta:   part.ReasoningDelta,
		ReasoningEnd:     part.ReasoningEnd,
		ResponseMetadata: part.ResponseMetadata,
		Finish:           part.Finish,
		ProviderMetadata: part.ProviderMetadata,
		Warnings:         part.Warnings,
		Raw:              part.Raw,
	}
	if part.Error != nil && part.Error.Err != nil {
		payload.Error = &streamErrorPayload{Message: part.Error.Err.Error()}
	}
	return json.Marshal(payload)
}

// PipeStream writes stream parts as SSE data to the response writer.
func PipeStream(ctx context.Context, writer http.ResponseWriter, stream *Stream[provider.StreamPart]) error {
	if stream == nil {
		return ErrNilStream
	}
	if writer == nil {
		return ErrNilResponseWriter
	}
	if ctx == nil {
		ctx = context.Background()
	}

	headers := writer.Header()
	headers.Set("Content-Type", "text/event-stream")
	headers.Set("Cache-Control", "no-cache")
	headers.Set("Connection", "keep-alive")

	done := make(chan struct{})
	defer close(done)
	go func() {
		select {
		case <-ctx.Done():
			stream.Close()
		case <-done:
		}
	}()

	defer stream.Close()
	for stream.Next() {
		payload, err := formatStreamPart(stream.Value())
		if err != nil {
			return err
		}
		if err := providerutils.WriteSSE(writer, providerutils.SSEEvent{Data: string(payload)}); err != nil {
			return err
		}
	}

	if err := stream.Err(); err != nil {
		return err
	}
	return nil
}
