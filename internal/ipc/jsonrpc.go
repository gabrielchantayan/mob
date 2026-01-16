package ipc

import (
	"encoding/json"
	"io"
	"sync"
)

// Client handles JSON-RPC communication with Claude Code
type Client struct {
	stdin  io.Writer
	stdout io.Reader
	mu     sync.Mutex
	nextID int
	enc    *json.Encoder
	dec    *json.Decoder
}

// NewClient creates a client from stdin/stdout pipes
func NewClient(stdin io.Writer, stdout io.Reader) *Client {
	return &Client{
		stdin:  stdin,
		stdout: stdout,
		nextID: 1,
		enc:    json.NewEncoder(stdin),
		dec:    json.NewDecoder(stdout),
	}
}

// Call sends a JSON-RPC request and waits for response
func (c *Client) Call(method string, params interface{}) (*Response, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	req := Request{
		JSONRPC: "2.0",
		ID:      c.nextID,
		Method:  method,
		Params:  params,
	}
	c.nextID++

	if err := c.enc.Encode(req); err != nil {
		return nil, err
	}

	var resp Response
	if err := c.dec.Decode(&resp); err != nil {
		return nil, err
	}

	return &resp, nil
}

// Send sends a request without waiting for response (notification)
func (c *Client) Send(method string, params interface{}) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	req := Request{
		JSONRPC: "2.0",
		ID:      c.nextID,
		Method:  method,
		Params:  params,
	}
	c.nextID++

	return c.enc.Encode(req)
}

// Receive reads the next message from stdout
func (c *Client) Receive() (*Response, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	var resp Response
	if err := c.dec.Decode(&resp); err != nil {
		return nil, err
	}

	return &resp, nil
}

// Close closes the client (no-op for now, but interface-ready)
func (c *Client) Close() error {
	return nil
}
