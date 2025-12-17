package ai

import (
	"errors"

	"github.com/bitop-dev/ai/internal/provider"
)

func mapProviderError(err error) error {
	if err == nil {
		return nil
	}
	var pe *provider.Error
	if errors.As(err, &pe) {
		return &Error{
			Provider:  pe.Provider,
			Code:      pe.Code,
			Status:    pe.Status,
			Message:   pe.Message,
			Retryable: pe.Retryable,
			Cause:     pe.Cause,
		}
	}
	return err
}
