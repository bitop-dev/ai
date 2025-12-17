package ai

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/bitop-dev/ai/internal/provider"
)

type testModel struct {
	provider string
	name     string
}

func (m testModel) Provider() string { return m.provider }
func (m testModel) Name() string     { return m.name }

type fakeProvider struct {
	mu sync.Mutex

	requests []provider.Request

	generate func(call int, req provider.Request) (provider.Response, error)
	stream   func(call int, req provider.Request) (provider.Stream, error)
}

func (p *fakeProvider) Generate(ctx context.Context, req provider.Request) (provider.Response, error) {
	_ = ctx
	p.mu.Lock()
	p.requests = append(p.requests, req)
	call := len(p.requests) - 1
	gen := p.generate
	p.mu.Unlock()
	if gen == nil {
		return provider.Response{}, fmt.Errorf("fakeProvider.Generate not configured")
	}
	return gen(call, req)
}

func (p *fakeProvider) Stream(ctx context.Context, req provider.Request) (provider.Stream, error) {
	_ = ctx
	p.mu.Lock()
	p.requests = append(p.requests, req)
	call := len(p.requests) - 1
	fn := p.stream
	p.mu.Unlock()
	if fn == nil {
		return nil, fmt.Errorf("fakeProvider.Stream not configured")
	}
	return fn(call, req)
}

func (p *fakeProvider) Requests() []provider.Request {
	p.mu.Lock()
	defer p.mu.Unlock()
	out := make([]provider.Request, len(p.requests))
	copy(out, p.requests)
	return out
}

func registerFakeProvider(t *testing.T, fp provider.Provider) string {
	t.Helper()
	name := "fake_" + strings.NewReplacer("/", "_", " ", "_").Replace(t.Name())
	if err := provider.Register(name, fp); err != nil {
		t.Fatalf("register provider: %v", err)
	}
	return name
}
