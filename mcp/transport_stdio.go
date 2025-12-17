package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"sync"
)

// StdioTransport connects to a local MCP server over stdin/stdout.
//
// Messages are framed as single-line JSON (one JSON-RPC message per line).
// This is intended for local development only.
type StdioTransport struct {
	Command string
	Args    []string
	Env     []string

	elicitationHandler func(ctx context.Context, req ElicitationRequest) (ElicitationResponse, error)

	mu     sync.Mutex
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout io.ReadCloser

	br *bufio.Reader
	bw *bufio.Writer

	nextID  int64
	pending map[int64]chan rpcResponse

	closed chan struct{}
	once   sync.Once
}

func (t *StdioTransport) startLocked() error {
	if t.cmd != nil {
		return nil
	}
	if t.Command == "" {
		return fmt.Errorf("mcp: stdio transport command is required")
	}
	cmd := exec.Command(t.Command, t.Args...)
	if len(t.Env) > 0 {
		cmd.Env = append([]string(nil), t.Env...)
	}

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		_ = stdin.Close()
		return err
	}
	// Server may write logs to stderr; inherit by default.

	if err := cmd.Start(); err != nil {
		_ = stdin.Close()
		_ = stdout.Close()
		return err
	}

	t.cmd = cmd
	t.stdin = stdin
	t.stdout = stdout
	t.br = bufio.NewReader(stdout)
	t.bw = bufio.NewWriter(stdin)
	t.pending = map[int64]chan rpcResponse{}
	t.closed = make(chan struct{})

	go t.readLoop()
	return nil
}

func (t *StdioTransport) readLoop() {
	for {
		line, err := t.br.ReadBytes('\n')
		if err != nil {
			t.failAll(err)
			return
		}
		// Trim trailing newline(s)
		for len(line) > 0 && (line[len(line)-1] == '\n' || line[len(line)-1] == '\r') {
			line = line[:len(line)-1]
		}
		if len(line) == 0 {
			continue
		}

		// Server can send responses or requests/notifications.
		var probe struct {
			Method string          `json:"method"`
			ID     json.RawMessage `json:"id"`
		}
		if err := json.Unmarshal(line, &probe); err != nil {
			t.failAll(err)
			return
		}

		if probe.Method != "" {
			t.handleServerRequest(line)
			continue
		}

		var resp rpcResponse
		if err := json.Unmarshal(line, &resp); err != nil {
			t.failAll(err)
			return
		}

		t.mu.Lock()
		ch := t.pending[resp.ID]
		if ch != nil {
			delete(t.pending, resp.ID)
		}
		t.mu.Unlock()
		if ch != nil {
			ch <- resp
			close(ch)
		}
	}
}

func (t *StdioTransport) handleServerRequest(line []byte) {
	var req struct {
		JSONRPC string          `json:"jsonrpc"`
		ID      *int64          `json:"id,omitempty"`
		Method  string          `json:"method"`
		Params  json.RawMessage `json:"params,omitempty"`
	}
	if err := json.Unmarshal(line, &req); err != nil {
		return
	}

	// Only elicitation is supported for now.
	if req.Method != "elicitation/create" {
		return
	}

	t.mu.Lock()
	h := t.elicitationHandler
	bw := t.bw
	t.mu.Unlock()
	if h == nil || bw == nil || req.ID == nil {
		return
	}

	var params elicitationCreateParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		_ = t.writeServerResponse(*req.ID, nil, &rpcError{Code: -32602, Message: "invalid params"})
		return
	}

	resp, err := h(context.Background(), ElicitationRequest{
		Message:         params.Message,
		RequestedSchema: params.RequestedSchema,
	})
	if err != nil {
		_ = t.writeServerResponse(*req.ID, nil, &rpcError{Code: -32000, Message: err.Error()})
		return
	}

	_ = t.writeServerResponse(*req.ID, resp, nil)
}

func (t *StdioTransport) writeServerResponse(id int64, result any, rpcErr *rpcError) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.bw == nil {
		return nil
	}
	msg := rpcResponse{JSONRPC: "2.0", ID: id}
	if rpcErr != nil {
		msg.Error = rpcErr
	} else {
		b, _ := json.Marshal(result)
		msg.Result = b
	}
	b, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	if _, err := t.bw.Write(b); err != nil {
		return err
	}
	if err := t.bw.WriteByte('\n'); err != nil {
		return err
	}
	return t.bw.Flush()
}

func (t *StdioTransport) failAll(err error) {
	t.once.Do(func() {
		close(t.closed)
	})
	t.mu.Lock()
	defer t.mu.Unlock()
	for id, ch := range t.pending {
		delete(t.pending, id)
		ch <- rpcResponse{JSONRPC: "2.0", ID: id, Error: &rpcError{Code: -32000, Message: err.Error()}}
		close(ch)
	}
}

func (t *StdioTransport) Call(ctx context.Context, req json.RawMessage) (json.RawMessage, error) {
	t.mu.Lock()
	if err := t.startLocked(); err != nil {
		t.mu.Unlock()
		return nil, err
	}
	// Assign an ID if needed (we expect caller to pass id, but stdio transport
	// benefits from being able to track pending requests).
	var parsed rpcRequest
	if err := json.Unmarshal(req, &parsed); err != nil {
		t.mu.Unlock()
		return nil, err
	}

	// Notifications (no id) and responses (no method) do not receive responses on stdio.
	if parsed.ID == nil || parsed.Method == "" {
		if _, err := t.bw.Write(req); err != nil {
			t.mu.Unlock()
			return nil, err
		}
		if err := t.bw.WriteByte('\n'); err != nil {
			t.mu.Unlock()
			return nil, err
		}
		if err := t.bw.Flush(); err != nil {
			t.mu.Unlock()
			return nil, err
		}
		t.mu.Unlock()
		return json.RawMessage(`{"jsonrpc":"2.0","id":0,"result":{}}`), nil
	}

	if *parsed.ID == 0 {
		t.nextID++
		id := t.nextID
		parsed.ID = &id
		b, _ := json.Marshal(parsed)
		req = b
	}
	id := *parsed.ID

	ch := make(chan rpcResponse, 1)
	t.pending[id] = ch

	if _, err := t.bw.Write(req); err != nil {
		delete(t.pending, id)
		t.mu.Unlock()
		return nil, err
	}
	if err := t.bw.WriteByte('\n'); err != nil {
		delete(t.pending, id)
		t.mu.Unlock()
		return nil, err
	}
	if err := t.bw.Flush(); err != nil {
		delete(t.pending, id)
		t.mu.Unlock()
		return nil, err
	}
	t.mu.Unlock()

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-t.closed:
		return nil, fmt.Errorf("mcp: stdio transport closed")
	case resp := <-ch:
		b, _ := json.Marshal(resp)
		return b, nil
	}
}

func (t *StdioTransport) Close() error {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.cmd == nil {
		return nil
	}
	_ = t.stdin.Close()
	_ = t.stdout.Close()
	_ = t.cmd.Process.Kill()
	_, _ = t.cmd.Process.Wait()
	t.cmd = nil
	t.once.Do(func() {
		close(t.closed)
	})
	return nil
}

func (t *StdioTransport) SetElicitationHandler(h func(ctx context.Context, req ElicitationRequest) (ElicitationResponse, error)) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.elicitationHandler = h
}
