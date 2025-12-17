package ai

import "errors"

type NoSuchToolError struct {
	ToolName string
}

func (e *NoSuchToolError) Error() string {
	if e == nil {
		return ""
	}
	return "no such tool: " + e.ToolName
}

func IsNoSuchTool(err error) bool {
	var e *NoSuchToolError
	return errors.As(err, &e)
}

type InvalidToolInputError struct {
	ToolName   string
	ToolCallID string
	Cause      error
}

func (e *InvalidToolInputError) Error() string {
	if e == nil {
		return ""
	}
	if e.Cause != nil {
		return "invalid tool input for " + e.ToolName + ": " + e.Cause.Error()
	}
	return "invalid tool input for " + e.ToolName
}

func (e *InvalidToolInputError) Unwrap() error { return e.Cause }

func IsInvalidToolInput(err error) bool {
	var e *InvalidToolInputError
	return errors.As(err, &e)
}

type ToolExecutionError struct {
	ToolName   string
	ToolCallID string
	Cause      error
}

func (e *ToolExecutionError) Error() string {
	if e == nil {
		return ""
	}
	if e.Cause != nil {
		return "tool execution failed for " + e.ToolName + ": " + e.Cause.Error()
	}
	return "tool execution failed for " + e.ToolName
}

func (e *ToolExecutionError) Unwrap() error { return e.Cause }
