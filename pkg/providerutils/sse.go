package providerutils

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// SSEEvent represents a single server-sent event payload.
type SSEEvent struct {
	Event string
	Data  string
	ID    string
	Retry *time.Duration
}

// SSEChunk captures each data line while parsing an SSE event.
type SSEChunk struct {
	Event string
	Data  string
	ID    string
}

// SSEParseOptions configures SSE parsing hooks.
type SSEParseOptions struct {
	OnEvent func(SSEEvent) error
	OnChunk func(SSEChunk) error
}

// ParseSSE reads SSE events from reader until EOF or error.
func ParseSSE(ctx context.Context, reader io.Reader, options SSEParseOptions) error {
	if reader == nil {
		return errors.New("sse parser requires a reader")
	}

	buffered := bufio.NewReader(reader)
	event := SSEEvent{}
	var dataLines []string

	reset := func() {
		event = SSEEvent{}
		dataLines = nil
	}

	dispatch := func() error {
		if event.Data == "" && len(dataLines) > 0 {
			event.Data = strings.Join(dataLines, "\n")
		}
		if event.Event == "" && event.Data == "" && event.ID == "" && event.Retry == nil {
			return nil
		}
		if options.OnEvent != nil {
			if err := options.OnEvent(event); err != nil {
				return err
			}
		}
		return nil
	}

	for {
		if ctx != nil && ctx.Err() != nil {
			return ctx.Err()
		}

		line, err := readSSELine(buffered)
		if err != nil {
			if errors.Is(err, io.EOF) {
				if len(dataLines) > 0 || event.Event != "" || event.ID != "" || event.Retry != nil {
					event.Data = strings.Join(dataLines, "\n")
					if dispatchErr := dispatch(); dispatchErr != nil {
						return dispatchErr
					}
				}
				return nil
			}
			return err
		}

		if line == "" {
			if err := dispatch(); err != nil {
				return err
			}
			reset()
			continue
		}

		if strings.HasPrefix(line, ":") {
			continue
		}

		field, value := splitSSELine(line)
		switch field {
		case "event":
			event.Event = value
		case "data":
			dataLines = append(dataLines, value)
			if options.OnChunk != nil {
				if err := options.OnChunk(SSEChunk{Event: event.Event, Data: value, ID: event.ID}); err != nil {
					return err
				}
			}
		case "id":
			event.ID = value
		case "retry":
			retry, parseErr := parseSSERetry(value)
			if parseErr != nil {
				return parseErr
			}
			event.Retry = &retry
		}
	}
}

// WriteSSE writes a single SSE event to the writer.
func WriteSSE(writer io.Writer, event SSEEvent) error {
	if writer == nil {
		return errors.New("sse writer is nil")
	}

	var builder strings.Builder
	if event.Event != "" {
		builder.WriteString("event: ")
		builder.WriteString(event.Event)
		builder.WriteString("\n")
	}
	if event.ID != "" {
		builder.WriteString("id: ")
		builder.WriteString(event.ID)
		builder.WriteString("\n")
	}
	if event.Retry != nil {
		builder.WriteString("retry: ")
		builder.WriteString(strconv.FormatInt(int64(event.Retry.Milliseconds()), 10))
		builder.WriteString("\n")
	}
	if event.Data != "" {
		for _, line := range strings.Split(event.Data, "\n") {
			builder.WriteString("data: ")
			builder.WriteString(line)
			builder.WriteString("\n")
		}
	}
	builder.WriteString("\n")

	if _, err := io.WriteString(writer, builder.String()); err != nil {
		return err
	}
	if flusher, ok := writer.(http.Flusher); ok {
		flusher.Flush()
	}
	return nil
}

// PipeSSE writes SSE events from a channel to an HTTP response writer.
func PipeSSE(ctx context.Context, writer http.ResponseWriter, events <-chan SSEEvent) error {
	if writer == nil {
		return errors.New("sse response writer is nil")
	}
	headers := writer.Header()
	headers.Set("Content-Type", "text/event-stream")
	headers.Set("Cache-Control", "no-cache")
	headers.Set("Connection", "keep-alive")

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case event, ok := <-events:
			if !ok {
				return nil
			}
			if err := WriteSSE(writer, event); err != nil {
				return err
			}
		}
	}
}

func readSSELine(reader *bufio.Reader) (string, error) {
	line, err := reader.ReadString('\n')
	if err != nil {
		if !errors.Is(err, io.EOF) {
			return "", err
		}
		if line == "" {
			return "", io.EOF
		}
	}
	return strings.TrimRight(line, "\r\n"), nil
}

func splitSSELine(line string) (string, string) {
	index := strings.IndexByte(line, ':')
	if index == -1 {
		return line, ""
	}
	field := line[:index]
	value := line[index+1:]
	if strings.HasPrefix(value, " ") {
		value = value[1:]
	}
	return field, value
}

func parseSSERetry(value string) (time.Duration, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return 0, fmt.Errorf("invalid retry value %q", value)
	}
	parsed, err := strconv.ParseInt(trimmed, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid retry value %q", value)
	}
	if parsed < 0 {
		return 0, fmt.Errorf("invalid retry value %q", value)
	}
	return time.Duration(parsed) * time.Millisecond, nil
}
