package mcp

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"sync"
)

type StdioConfig struct {
	Command string
	Args    []string
	Env     map[string]string
	Stderr  io.Writer
	Cwd     string
}

type StdioTransport struct {
	config         StdioConfig
	handlers       TransportHandlers
	process        stdioProcess
	processFactory func(context.Context, StdioConfig) (stdioProcess, error)
	cancel         context.CancelFunc
	mu             sync.Mutex
	closing        bool
}

func NewStdioTransport(config StdioConfig) *StdioTransport {
	return &StdioTransport{config: config}
}

func (transport *StdioTransport) Start(ctx context.Context) error {
	transport.mu.Lock()
	if transport.process != nil {
		transport.mu.Unlock()
		return fmt.Errorf("mcp stdio transport already started")
	}
	factory := transport.processFactory
	if factory == nil {
		factory = newExecProcess
	}
	startCtx, cancel := context.WithCancel(ctx)
	process, err := factory(startCtx, transport.config)
	if err != nil {
		cancel()
		transport.mu.Unlock()
		return err
	}
	if err := process.Start(); err != nil {
		cancel()
		transport.mu.Unlock()
		return err
	}
	transport.process = process
	transport.cancel = cancel
	transport.closing = false
	transport.mu.Unlock()

	go transport.readLoop(process.Stdout())
	go transport.waitLoop(process)
	return nil
}

func (transport *StdioTransport) Send(ctx context.Context, message Message) error {
	transport.mu.Lock()
	if transport.process == nil || transport.closing {
		transport.mu.Unlock()
		return fmt.Errorf("mcp stdio transport not connected")
	}
	stdin := transport.process.Stdin()
	transport.mu.Unlock()

	payload, err := json.Marshal(message)
	if err != nil {
		return newResponseError("mcp stdio failed to encode message", err)
	}
	payload = append(payload, '\n')

	done := make(chan error, 1)
	go func() {
		_, writeErr := stdin.Write(payload)
		done <- writeErr
	}()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case writeErr := <-done:
		if writeErr != nil {
			return writeErr
		}
		return nil
	}
}

func (transport *StdioTransport) Close() error {
	transport.mu.Lock()
	if transport.process == nil || transport.closing {
		transport.mu.Unlock()
		return nil
	}
	transport.closing = true
	process := transport.process
	cancel := transport.cancel
	transport.mu.Unlock()

	if cancel != nil {
		cancel()
	}
	if process != nil {
		_ = process.Kill()
	}
	return nil
}

func (transport *StdioTransport) SetHandlers(handlers TransportHandlers) {
	transport.handlers = handlers
}

func (transport *StdioTransport) readLoop(reader io.Reader) {
	buffered := bufio.NewReader(reader)
	for {
		line, err := buffered.ReadBytes('\n')
		if len(line) > 0 {
			if message, parseErr := deserializeMessage(line); parseErr != nil {
				if transport.handlers.OnError != nil {
					transport.handlers.OnError(parseErr)
				}
			} else if transport.handlers.OnMessage != nil {
				transport.handlers.OnMessage(message)
			}
		}
		if err != nil {
			if err != io.EOF && transport.handlers.OnError != nil {
				transport.handlers.OnError(err)
			}
			return
		}
	}
}

func (transport *StdioTransport) waitLoop(process stdioProcess) {
	err := process.Wait()
	transport.mu.Lock()
	transport.process = nil
	closing := transport.closing
	transport.mu.Unlock()

	if err != nil && !closing && transport.handlers.OnError != nil {
		transport.handlers.OnError(err)
	}
	if transport.handlers.OnClose != nil {
		transport.handlers.OnClose()
	}
}

type stdioProcess interface {
	Stdin() io.WriteCloser
	Stdout() io.ReadCloser
	Start() error
	Wait() error
	Kill() error
}

type execProcess struct {
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout io.ReadCloser
}

func newExecProcess(ctx context.Context, config StdioConfig) (stdioProcess, error) {
	if config.Command == "" {
		return nil, fmt.Errorf("mcp stdio transport requires a command")
	}
	cmd := exec.CommandContext(ctx, config.Command, config.Args...)
	cmd.Env = mergeEnvironment(config.Env)
	if config.Cwd != "" {
		cmd.Dir = config.Cwd
	}
	if config.Stderr != nil {
		cmd.Stderr = config.Stderr
	} else {
		cmd.Stderr = os.Stderr
	}
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		_ = stdin.Close()
		return nil, err
	}
	return &execProcess{cmd: cmd, stdin: stdin, stdout: stdout}, nil
}

func (process *execProcess) Stdin() io.WriteCloser {
	return process.stdin
}

func (process *execProcess) Stdout() io.ReadCloser {
	return process.stdout
}

func (process *execProcess) Start() error {
	return process.cmd.Start()
}

func (process *execProcess) Wait() error {
	return process.cmd.Wait()
}

func (process *execProcess) Kill() error {
	if process.cmd.Process == nil {
		return nil
	}
	return process.cmd.Process.Kill()
}

type jsonRPCEnvelope struct {
	JSONRPC string              `json:"jsonrpc"`
	ID      *json.RawMessage    `json:"id,omitempty"`
	Method  string              `json:"method,omitempty"`
	Params  json.RawMessage     `json:"params,omitempty"`
	Result  json.RawMessage     `json:"result,omitempty"`
	Error   *JSONRPCErrorObject `json:"error,omitempty"`
}

func deserializeMessage(line []byte) (Message, error) {
	trimmed := bytes.TrimSpace(line)
	if len(trimmed) == 0 {
		return nil, fmt.Errorf("mcp stdio received empty message")
	}
	var envelope jsonRPCEnvelope
	if err := json.Unmarshal(trimmed, &envelope); err != nil {
		return nil, newResponseError("mcp stdio failed to decode message", err)
	}
	if envelope.JSONRPC == "" {
		return nil, newResponseError("mcp stdio missing jsonrpc", nil)
	}
	if envelope.Error != nil {
		messageID, err := parseMessageID(envelope.ID)
		if err != nil {
			return nil, err
		}
		return JSONRPCError{JSONRPC: envelope.JSONRPC, ID: messageID, Error: *envelope.Error}, nil
	}
	if envelope.Method != "" {
		params, err := decodePayload(envelope.Params)
		if err != nil {
			return nil, err
		}
		if envelope.ID != nil {
			messageID, err := parseMessageID(envelope.ID)
			if err != nil {
				return nil, err
			}
			return JSONRPCRequest{JSONRPC: envelope.JSONRPC, ID: messageID, Method: envelope.Method, Params: params}, nil
		}
		return JSONRPCNotification{JSONRPC: envelope.JSONRPC, Method: envelope.Method, Params: params}, nil
	}
	if envelope.Result != nil {
		messageID, err := parseMessageID(envelope.ID)
		if err != nil {
			return nil, err
		}
		result, err := decodePayload(envelope.Result)
		if err != nil {
			return nil, err
		}
		return JSONRPCResponse{JSONRPC: envelope.JSONRPC, ID: messageID, Result: result}, nil
	}
	return nil, newResponseError("mcp stdio message type not recognized", nil)
}

func decodePayload(payload json.RawMessage) (any, error) {
	if len(payload) == 0 {
		return nil, nil
	}
	var value any
	if err := json.Unmarshal(payload, &value); err != nil {
		return nil, newResponseError("mcp stdio payload decode failed", err)
	}
	return value, nil
}

func parseMessageID(raw *json.RawMessage) (int64, error) {
	if raw == nil {
		return 0, newResponseError("mcp stdio message missing id", nil)
	}
	var value any
	if err := json.Unmarshal(*raw, &value); err != nil {
		return 0, newResponseError("mcp stdio failed to decode id", err)
	}
	switch typed := value.(type) {
	case float64:
		return int64(typed), nil
	case string:
		parsed, err := strconv.ParseInt(typed, 10, 64)
		if err != nil {
			return 0, newResponseError("mcp stdio invalid id", err)
		}
		return parsed, nil
	default:
		return 0, newResponseError("mcp stdio invalid id", nil)
	}
}

func mergeEnvironment(custom map[string]string) []string {
	env := map[string]string{}
	if custom != nil {
		for key, value := range custom {
			env[key] = value
		}
	}
	for _, key := range defaultEnvKeys() {
		value := os.Getenv(key)
		if value == "" {
			continue
		}
		if strings.HasPrefix(value, "()") {
			continue
		}
		if _, exists := env[key]; !exists {
			env[key] = value
		}
	}
	envList := make([]string, 0, len(env))
	for key, value := range env {
		envList = append(envList, fmt.Sprintf("%s=%s", key, value))
	}
	return envList
}

func defaultEnvKeys() []string {
	if runtime.GOOS == "windows" {
		return []string{
			"APPDATA",
			"HOMEDRIVE",
			"HOMEPATH",
			"LOCALAPPDATA",
			"PATH",
			"PROCESSOR_ARCHITECTURE",
			"SYSTEMDRIVE",
			"SYSTEMROOT",
			"TEMP",
			"USERNAME",
			"USERPROFILE",
		}
	}
	return []string{"HOME", "LOGNAME", "PATH", "SHELL", "TERM", "USER"}
}
