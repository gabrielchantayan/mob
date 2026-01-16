package mcp

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sync"

	"github.com/gabe/mob/internal/agent"
	"github.com/gabe/mob/internal/registry"
	"github.com/gabe/mob/internal/storage"
)

// Server implements an MCP server over stdio
type Server struct {
	registry  *registry.Registry
	spawner   *agent.Spawner
	beadStore *storage.BeadStore
	mobDir    string
	tools     map[string]*Tool
	taskWg    sync.WaitGroup // Track background tasks
}

// NewServer creates a new MCP server
func NewServer(reg *registry.Registry, spawner *agent.Spawner, beadStore *storage.BeadStore, mobDir string) *Server {
	s := &Server{
		registry:  reg,
		spawner:   spawner,
		beadStore: beadStore,
		mobDir:    mobDir,
		tools:     make(map[string]*Tool),
	}

	// Register all tools
	for _, tool := range GetTools() {
		s.tools[tool.Name] = tool
	}

	return s
}

// JSON-RPC 2.0 structures
type jsonRPCRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      interface{}     `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type jsonRPCResponse struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id,omitempty"`
	Result  interface{} `json:"result,omitempty"`
	Error   *rpcError   `json:"error,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// MCP protocol structures
type initializeParams struct {
	ProtocolVersion string     `json:"protocolVersion"`
	ClientInfo      clientInfo `json:"clientInfo"`
}

type clientInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

type initializeResult struct {
	ProtocolVersion string       `json:"protocolVersion"`
	ServerInfo      serverInfo   `json:"serverInfo"`
	Capabilities    capabilities `json:"capabilities"`
}

type serverInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

type capabilities struct {
	Tools *toolsCapability `json:"tools,omitempty"`
}

type toolsCapability struct {
	ListChanged bool `json:"listChanged,omitempty"`
}

type toolsListResult struct {
	Tools []toolDefinition `json:"tools"`
}

type toolDefinition struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema json.RawMessage `json:"inputSchema"`
}

type toolCallParams struct {
	Name      string                 `json:"name"`
	Arguments map[string]interface{} `json:"arguments,omitempty"`
}

type toolCallResult struct {
	Content []contentBlock `json:"content"`
	IsError bool           `json:"isError,omitempty"`
}

type contentBlock struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// Run starts the MCP server, reading from stdin and writing to stdout
func (s *Server) Run() error {
	reader := bufio.NewReader(os.Stdin)
	writer := os.Stdout

	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				// Wait for any background tasks to complete before exiting
				s.taskWg.Wait()
				return nil
			}
			return fmt.Errorf("error reading input: %w", err)
		}

		var req jsonRPCRequest
		if err := json.Unmarshal([]byte(line), &req); err != nil {
			s.writeError(writer, nil, -32700, "Parse error")
			continue
		}

		response := s.handleRequest(&req)
		if response != nil {
			s.writeResponse(writer, response)
		}
	}
}

func (s *Server) handleRequest(req *jsonRPCRequest) *jsonRPCResponse {
	switch req.Method {
	case "initialize":
		return s.handleInitialize(req)
	case "initialized":
		// Notification, no response
		return nil
	case "tools/list":
		return s.handleToolsList(req)
	case "tools/call":
		return s.handleToolsCall(req)
	default:
		return &jsonRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error: &rpcError{
				Code:    -32601,
				Message: fmt.Sprintf("Method not found: %s", req.Method),
			},
		}
	}
}

func (s *Server) handleInitialize(req *jsonRPCRequest) *jsonRPCResponse {
	return &jsonRPCResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result: initializeResult{
			ProtocolVersion: "2024-11-05",
			ServerInfo: serverInfo{
				Name:    "mob-tools",
				Version: "1.0.0",
			},
			Capabilities: capabilities{
				Tools: &toolsCapability{},
			},
		},
	}
}

func (s *Server) handleToolsList(req *jsonRPCRequest) *jsonRPCResponse {
	tools := make([]toolDefinition, 0, len(s.tools))
	for _, tool := range s.tools {
		schemaBytes, _ := json.Marshal(tool.InputSchema)
		tools = append(tools, toolDefinition{
			Name:        tool.Name,
			Description: tool.Description,
			InputSchema: schemaBytes,
		})
	}

	return &jsonRPCResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result: toolsListResult{
			Tools: tools,
		},
	}
}

func (s *Server) handleToolsCall(req *jsonRPCRequest) *jsonRPCResponse {
	var params toolCallParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return &jsonRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error: &rpcError{
				Code:    -32602,
				Message: "Invalid params",
			},
		}
	}

	tool, ok := s.tools[params.Name]
	if !ok {
		return &jsonRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error: &rpcError{
				Code:    -32602,
				Message: fmt.Sprintf("Unknown tool: %s", params.Name),
			},
		}
	}

	// Execute the tool
	ctx := &ToolContext{
		Registry:  s.registry,
		Spawner:   s.spawner,
		BeadStore: s.beadStore,
		MobDir:    s.mobDir,
		TaskWg:    &s.taskWg,
	}

	result, err := tool.Handler(ctx, params.Arguments)
	if err != nil {
		return &jsonRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result: toolCallResult{
				Content: []contentBlock{
					{Type: "text", Text: fmt.Sprintf("Error: %s", err.Error())},
				},
				IsError: true,
			},
		}
	}

	return &jsonRPCResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result: toolCallResult{
			Content: []contentBlock{
				{Type: "text", Text: result},
			},
		},
	}
}

func (s *Server) writeResponse(w io.Writer, resp *jsonRPCResponse) {
	data, _ := json.Marshal(resp)
	fmt.Fprintln(w, string(data))
}

func (s *Server) writeError(w io.Writer, id interface{}, code int, message string) {
	resp := &jsonRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Error: &rpcError{
			Code:    code,
			Message: message,
		},
	}
	s.writeResponse(w, resp)
}
