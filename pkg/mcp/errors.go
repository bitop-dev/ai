package mcp

import (
	"fmt"

	"github.com/vercel/ai-sdk-go/pkg/provider"
)

type RPCError struct {
	Code    int
	Message string
	Data    any
}

func (err RPCError) Error() string {
	if err.Message != "" {
		return err.Message
	}
	return fmt.Sprintf("mcp error %d", err.Code)
}

func MapRPCError(err RPCError) error {
	switch err.Code {
	case -32700:
		return provider.NewInvalidResponseDataError("mcp parse error", err)
	case -32600:
		return provider.NewInvalidRequestError("mcp invalid request", err)
	case -32601:
		return provider.NewUnsupportedFunctionalityError("mcp method not found", err, "method")
	case -32602:
		return provider.NewInvalidRequestError("mcp invalid params", err)
	case -32603:
		return provider.NewInternalServerError("mcp internal error", err)
	default:
		return provider.NewApiCallError("mcp request failed", err)
	}
}

func newCapabilityError(feature string) error {
	return provider.NewUnsupportedFunctionalityError(
		fmt.Sprintf("mcp server does not support %s", feature),
		nil,
		feature,
	)
}

func newResponseError(message string, cause error) error {
	return provider.NewInvalidResponseDataError(message, cause)
}
