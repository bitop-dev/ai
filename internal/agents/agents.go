package agents

import (
	"context"

	"github.com/bitop-dev/ai/internal/provider"
	"github.com/bitop-dev/ai/internal/text"
	"github.com/bitop-dev/ai/internal/tools"
)

func Generate(ctx context.Context, p provider.Provider, req provider.Request, exec tools.Executor, opts text.Options) (text.GenerateResult, error) {
	return text.Generate(ctx, p, req, exec, opts)
}

func NewStream(ctx context.Context, p provider.Provider, req provider.Request, exec tools.Executor, opts text.Options, onDelta func(provider.Delta)) *text.Stream {
	return text.NewStream(ctx, p, req, exec, opts, onDelta)
}
