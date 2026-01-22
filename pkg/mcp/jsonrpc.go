package mcp

type Message interface {
	isMCPMessage()
}

type JSONRPCRequest struct {
	JSONRPC string `json:"jsonrpc"`
	ID      int64  `json:"id"`
	Method  string `json:"method"`
	Params  any    `json:"params,omitempty"`
}

func (JSONRPCRequest) isMCPMessage() {}

type JSONRPCNotification struct {
	JSONRPC string `json:"jsonrpc"`
	Method  string `json:"method"`
	Params  any    `json:"params,omitempty"`
}

func (JSONRPCNotification) isMCPMessage() {}

type JSONRPCResponse struct {
	JSONRPC string `json:"jsonrpc"`
	ID      int64  `json:"id"`
	Result  any    `json:"result"`
}

func (JSONRPCResponse) isMCPMessage() {}

type JSONRPCErrorObject struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

type JSONRPCError struct {
	JSONRPC string             `json:"jsonrpc"`
	ID      int64              `json:"id"`
	Error   JSONRPCErrorObject `json:"error"`
}

func (JSONRPCError) isMCPMessage() {}
