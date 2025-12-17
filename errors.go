package ai

import (
	"context"
	"errors"
)

type Error struct {
	Provider  string
	Code      string
	Status    int
	Message   string
	Retryable bool
	Cause     error
}

func (e *Error) Error() string {
	if e == nil {
		return ""
	}
	if e.Provider != "" && e.Message != "" {
		return e.Provider + ": " + e.Message
	}
	if e.Message != "" {
		return e.Message
	}
	if e.Provider != "" {
		return e.Provider + ": error"
	}
	return "error"
}

func (e *Error) Unwrap() error { return e.Cause }

func IsRateLimited(err error) bool {
	var e *Error
	return errors.As(err, &e) && (e.Status == 429 || e.Code == "rate_limited")
}

func IsAuth(err error) bool {
	var e *Error
	return errors.As(err, &e) && (e.Status == 401 || e.Status == 403 || e.Code == "unauthorized")
}

func IsTimeout(err error) bool {
	var e *Error
	if errors.As(err, &e) && e.Code == "timeout" {
		return true
	}
	return errors.Is(err, context.DeadlineExceeded)
}

func IsCanceled(err error) bool {
	var e *Error
	if errors.As(err, &e) && e.Code == "canceled" {
		return true
	}
	return errors.Is(err, context.Canceled)
}
