package ipc

import (
	"encoding/json"
	"fmt"
	"time"
)

// Request represents a JSON-RPC 2.0 request
type Request struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      int         `json:"id"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params,omitempty"`
}

// Response represents a JSON-RPC 2.0 response
type Response struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      int             `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *RPCError       `json:"error,omitempty"`
}

// RPCError represents a JSON-RPC error
type RPCError struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// Error implements the error interface for RPCError
func (e *RPCError) Error() string {
	return fmt.Sprintf("rpc error: code=%d, message=%s", e.Code, e.Message)
}

// AgentOutputMsg represents a streaming output message from an agent
type AgentOutputMsg struct {
	AgentID   string    `json:"agent_id"`
	AgentName string    `json:"agent_name"`
	Line      string    `json:"line"`
	Timestamp time.Time `json:"timestamp"`
	Stream    string    `json:"stream"` // "stdout" or "stderr"
}
