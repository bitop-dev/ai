package mcp

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"sync"
	"testing"
	"time"
)

type fakeProcess struct {
	stdinReader  *io.PipeReader
	stdinWriter  *io.PipeWriter
	stdoutReader *io.PipeReader
	stdoutWriter *io.PipeWriter
	waitCh       chan struct{}
	closeOnce    sync.Once
}

func newFakeProcess() *fakeProcess {
	stdinReader, stdinWriter := io.Pipe()
	stdoutReader, stdoutWriter := io.Pipe()
	return &fakeProcess{
		stdinReader:  stdinReader,
		stdinWriter:  stdinWriter,
		stdoutReader: stdoutReader,
		stdoutWriter: stdoutWriter,
		waitCh:       make(chan struct{}),
	}
}

func (process *fakeProcess) Stdin() io.WriteCloser {
	return process.stdinWriter
}

func (process *fakeProcess) Stdout() io.ReadCloser {
	return process.stdoutReader
}

func (process *fakeProcess) Start() error {
	return nil
}

func (process *fakeProcess) Wait() error {
	<-process.waitCh
	return nil
}

func (process *fakeProcess) Kill() error {
	process.closeOnce.Do(func() {
		_ = process.stdoutWriter.Close()
		_ = process.stdinWriter.Close()
		close(process.waitCh)
	})
	return nil
}

func TestStdioTransportStartAlreadyStarted(t *testing.T) {
	transport := NewStdioTransport(StdioConfig{Command: "test"})
	transport.processFactory = func(ctx context.Context, config StdioConfig) (stdioProcess, error) {
		return newFakeProcess(), nil
	}

	if err := transport.Start(context.Background()); err != nil {
		t.Fatalf("start transport: %v", err)
	}
	if err := transport.Start(context.Background()); err == nil {
		t.Fatalf("expected error for second start")
	}
	_ = transport.Close()
}

func TestStdioTransportSendAndReceive(t *testing.T) {
	process := newFakeProcess()
	transport := NewStdioTransport(StdioConfig{Command: "test"})
	transport.processFactory = func(ctx context.Context, config StdioConfig) (stdioProcess, error) {
		return process, nil
	}

	messageCh := make(chan Message, 1)
	transport.SetHandlers(TransportHandlers{
		OnMessage: func(message Message) {
			messageCh <- message
		},
	})

	if err := transport.Start(context.Background()); err != nil {
		t.Fatalf("start transport: %v", err)
	}

	send := JSONRPCRequest{JSONRPC: "2.0", ID: 1, Method: "ping", Params: map[string]any{"ok": true}}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	readDone := make(chan string, 1)
	go func() {
		reader := bufio.NewReader(process.stdinReader)
		line, _ := reader.ReadBytes('\n')
		readDone <- string(line)
	}()
	if err := transport.Send(ctx, send); err != nil {
		t.Fatalf("send: %v", err)
	}
	select {
	case line := <-readDone:
		var decoded JSONRPCRequest
		if err := json.Unmarshal(bytes.TrimSpace([]byte(line)), &decoded); err != nil {
			t.Fatalf("decode sent message: %v", err)
		}
		if decoded.Method != "ping" {
			t.Fatalf("expected method ping, got %q", decoded.Method)
		}
	case <-time.After(time.Second):
		t.Fatalf("timed out waiting for sent message")
	}

	response := JSONRPCResponse{JSONRPC: "2.0", ID: 1, Result: map[string]any{"pong": true}}
	payload, err := json.Marshal(response)
	if err != nil {
		t.Fatalf("marshal response: %v", err)
	}
	if _, err := process.stdoutWriter.Write(append(payload, '\n')); err != nil {
		t.Fatalf("write response: %v", err)
	}

	select {
	case message := <-messageCh:
		received, ok := message.(JSONRPCResponse)
		if !ok {
			t.Fatalf("expected JSONRPCResponse, got %T", message)
		}
		if received.ID != 1 {
			t.Fatalf("expected id 1, got %d", received.ID)
		}
	case <-time.After(time.Second):
		t.Fatalf("timed out waiting for message")
	}
	_ = transport.Close()
}

func TestStdioTransportPartialMessage(t *testing.T) {
	process := newFakeProcess()
	transport := NewStdioTransport(StdioConfig{Command: "test"})
	transport.processFactory = func(ctx context.Context, config StdioConfig) (stdioProcess, error) {
		return process, nil
	}

	messageCh := make(chan JSONRPCNotification, 1)
	transport.SetHandlers(TransportHandlers{
		OnMessage: func(message Message) {
			notification, ok := message.(JSONRPCNotification)
			if ok {
				messageCh <- notification
			}
		},
	})

	if err := transport.Start(context.Background()); err != nil {
		t.Fatalf("start transport: %v", err)
	}

	notification := JSONRPCNotification{JSONRPC: "2.0", Method: "notify", Params: map[string]any{"ok": true}}
	payload, err := json.Marshal(notification)
	if err != nil {
		t.Fatalf("marshal notification: %v", err)
	}
	mid := len(payload) / 2
	if _, err := process.stdoutWriter.Write(payload[:mid]); err != nil {
		t.Fatalf("write payload part: %v", err)
	}
	if _, err := process.stdoutWriter.Write(append(payload[mid:], '\n')); err != nil {
		t.Fatalf("write payload remainder: %v", err)
	}

	select {
	case message := <-messageCh:
		if message.Method != "notify" {
			t.Fatalf("expected notify method, got %q", message.Method)
		}
	case <-time.After(time.Second):
		t.Fatalf("timed out waiting for partial message")
	}
	_ = transport.Close()
}

func TestStdioTransportClose(t *testing.T) {
	process := newFakeProcess()
	transport := NewStdioTransport(StdioConfig{Command: "test"})
	transport.processFactory = func(ctx context.Context, config StdioConfig) (stdioProcess, error) {
		return process, nil
	}

	closed := make(chan struct{}, 1)
	transport.SetHandlers(TransportHandlers{
		OnClose: func() {
			closed <- struct{}{}
		},
	})

	if err := transport.Start(context.Background()); err != nil {
		t.Fatalf("start transport: %v", err)
	}
	if err := transport.Close(); err != nil {
		t.Fatalf("close transport: %v", err)
	}
	select {
	case <-closed:
	case <-time.After(time.Second):
		t.Fatalf("timed out waiting for close")
	}
}
