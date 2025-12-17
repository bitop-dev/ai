package provider

import (
	"context"
	"fmt"
	"sync"
)

type Provider interface {
	Generate(ctx context.Context, req Request) (Response, error)
	Stream(ctx context.Context, req Request) (Stream, error)
}

type Request struct {
	Model string

	Messages []Message
	Tools    []ToolDefinition

	// ProviderData may carry provider-specific wiring (e.g. a client handle).
	// Providers must treat unknown types as an error.
	ProviderData any

	MaxTokens   *int
	Temperature *float32
	TopP        *float32
	Stop        []string

	Metadata map[string]string
}

type Response struct {
	Message      Message
	Usage        Usage
	FinishReason FinishReason
}

type Stream interface {
	Next() bool
	Delta() Delta
	Final() *Response
	Err() error
	Close() error
}

type Registry struct {
	mu        sync.RWMutex
	providers map[string]Provider
}

func NewRegistry() *Registry {
	return &Registry{providers: map[string]Provider{}}
}

func (r *Registry) Register(name string, p Provider) error {
	if name == "" {
		return fmt.Errorf("provider name is required")
	}
	if p == nil {
		return fmt.Errorf("provider %q is nil", name)
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.providers[name]; exists {
		return fmt.Errorf("provider %q already registered", name)
	}

	r.providers[name] = p
	return nil
}

func (r *Registry) Get(name string) (Provider, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	p, ok := r.providers[name]
	return p, ok
}

var defaultRegistry = NewRegistry()

func Register(name string, p Provider) error {
	return defaultRegistry.Register(name, p)
}

func Get(name string) (Provider, bool) {
	return defaultRegistry.Get(name)
}
