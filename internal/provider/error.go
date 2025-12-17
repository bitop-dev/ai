package provider

import "fmt"

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
		return fmt.Sprintf("%s: %s", e.Provider, e.Message)
	}
	if e.Message != "" {
		return e.Message
	}
	if e.Provider != "" {
		return fmt.Sprintf("%s: error", e.Provider)
	}
	return "error"
}

func (e *Error) Unwrap() error { return e.Cause }
