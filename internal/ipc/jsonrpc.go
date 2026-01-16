package ipc

// Client handles JSON-RPC communication with Claude Code
type Client struct {
	// Will hold stdin/stdout pipes
}

// New creates a new IPC client
func New() *Client {
	return &Client{}
}
