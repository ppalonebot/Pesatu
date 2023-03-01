package jsonrpc2

import (
	"encoding/json"
)

type RPCRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
	ID      string          `json:"id,omitempty"`
	Notif   bool            `json:"-"`
}

type RPCResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *RPCError       `json:"error,omitempty"`
	ID      string          `json:"id,omitempty"`
}

type RPCError struct {
	Code    int                `json:"code"`
	Message string             `json:"message"`
	Params  []*InputFieldError `json:"params,omitempty"`
}

type InputFieldError struct {
	Error string `json:"error"`
	Field string `json:"field"`
}
