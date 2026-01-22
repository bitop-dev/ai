package mcp

import "context"

type TransportHandlers struct {
	OnMessage func(Message)
	OnError   func(error)
	OnClose   func()
}

type Transport interface {
	Start(ctx context.Context) error
	Send(ctx context.Context, message Message) error
	Close() error
	SetHandlers(handlers TransportHandlers)
}
