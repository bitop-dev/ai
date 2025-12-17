package openai

import "github.com/bitop-dev/ai/internal/provider"

func init() {
	_ = provider.Register("openai", &Provider{})
}
