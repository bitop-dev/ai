package ai

import (
	"errors"

	"github.com/bitop-dev/ai/pkg/provider"
	"github.com/bitop-dev/ai/pkg/registry"
)

// ErrNilRegistry indicates a nil registry was provided.
var ErrNilRegistry = errors.New("registry is nil")

// ResolveModel resolves a language model from a provider registry.
func ResolveModel(modelRegistry registry.ProviderRegistry, id string) (provider.LanguageModelV3, error) {
	if modelRegistry == nil {
		return nil, ErrNilRegistry
	}
	return modelRegistry.LanguageModel(id)
}
