package mcp

import "fmt"

type RPCError struct {
	Code    int64
	Message string
	Data    []byte
}

func (e *RPCError) Error() string {
	if e == nil {
		return ""
	}
	if e.Message != "" {
		return fmt.Sprintf("mcp rpc error %d: %s", e.Code, e.Message)
	}
	return fmt.Sprintf("mcp rpc error %d", e.Code)
}
