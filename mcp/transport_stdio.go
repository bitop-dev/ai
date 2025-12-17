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
	if parsed.ID == 0 {
		t.nextID++
		parsed.ID = t.nextID
		b, _ := json.Marshal(parsed)
		req = b
	}

	ch := make(chan rpcResponse, 1)
	t.pending[parsed.ID] = ch

	if _, err := t.bw.Write(req); err != nil {
		delete(t.pending, parsed.ID)
		t.mu.Unlock()
		return nil, err
	}
	if err := t.bw.WriteByte('\n'); err != nil {
		delete(t.pending, parsed.ID)
		t.mu.Unlock()
		return nil, err
	}
	if err := t.bw.Flush(); err != nil {
		delete(t.pending, parsed.ID)
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
