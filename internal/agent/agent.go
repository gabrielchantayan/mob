package agent

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"
)

// AgentType represents the type of agent
type AgentType string

const (
	AgentTypeUnderboss AgentType = "underboss"
	AgentTypeSoldati   AgentType = "soldati"
	AgentTypeAssociate AgentType = "associate"
)

// Agent represents a Claude Code agent that can send messages
// Uses per-call spawning with --resume for session continuity
type Agent struct {
	ID           string
	Type         AgentType
	Name         string // e.g., "vinnie" for soldati
	Turf         string // project this agent works on
	WorkDir      string // working directory for Claude
	StartedAt    time.Time
	SessionID    string // Claude session ID for --resume
	SystemPrompt string // System prompt injected on first call
	MCPConfig    string // Path to MCP config JSON file
	Model        string // Model to use (e.g., "sonnet", "opus") - passed as --model flag
	spawner      *Spawner
	mu           sync.Mutex
}

// ContentBlockType represents the type of content in a response
type ContentBlockType string

const (
	ContentTypeText       ContentBlockType = "text"
	ContentTypeThinking   ContentBlockType = "thinking"
	ContentTypeToolUse    ContentBlockType = "tool_use"
	ContentTypeToolResult ContentBlockType = "tool_result"
)

// ChatContentBlock represents a piece of content in a chat response
type ChatContentBlock struct {
	Type    ContentBlockType
	Text    string // For text and thinking
	Name    string // For tool_use (tool name)
	Input   string // For tool_use (tool input as string)
	ID      string // For tool_use (tool_use_id)
	Summary string // For thinking (summary header)
	Index   int    // Block index for streaming updates
}

// ChatResponse represents a complete response from Claude
type ChatResponse struct {
	Blocks       []ChatContentBlock
	SessionID    string
	Model        string
	DurationMs   int64
	TotalCost    float64
	InputTokens  int
	OutputTokens int
}

// GetText returns all text content concatenated
func (r *ChatResponse) GetText() string {
	var parts []string
	for _, b := range r.Blocks {
		if b.Type == ContentTypeText {
			parts = append(parts, b.Text)
		}
	}
	return strings.Join(parts, "")
}

// StreamCallback is called for each content update during streaming
type StreamCallback func(block ChatContentBlock)

// StreamMessage represents a message in Claude's stream-json output
type StreamMessage struct {
	Type         string         `json:"type"`
	Subtype      string         `json:"subtype,omitempty"`
	SessionID    string         `json:"session_id,omitempty"`
	Message      *ClaudeMessage `json:"message,omitempty"`
	Event        *StreamEvent   `json:"event,omitempty"`
	Result       string         `json:"result,omitempty"`
	IsError      bool           `json:"is_error,omitempty"`
	DurationMs   int64          `json:"duration_ms,omitempty"`
	TotalCostUSD float64        `json:"total_cost_usd,omitempty"`
	Usage        *UsageInfo     `json:"usage,omitempty"`
}

// StreamEvent represents streaming events
type StreamEvent struct {
	Type         string        `json:"type"`
	Index        int           `json:"index,omitempty"`
	ContentBlock *ContentBlock `json:"content_block,omitempty"`
	Delta        *ContentDelta `json:"delta,omitempty"`
}

// ContentDelta represents incremental content updates
type ContentDelta struct {
	Type    string `json:"type"`
	Text    string `json:"text,omitempty"`
	Summary string `json:"summary,omitempty"`
}

// ClaudeMessage represents the message field in assistant responses
type ClaudeMessage struct {
	Model   string         `json:"model,omitempty"`
	Content []ContentBlock `json:"content,omitempty"`
}

// ContentBlock represents a content block in Claude's response
type ContentBlock struct {
	Type      string                 `json:"type"`
	Text      string                 `json:"text,omitempty"`
	Name      string                 `json:"name,omitempty"`
	ID        string                 `json:"id,omitempty"`
	Input     map[string]interface{} `json:"input,omitempty"`
	Summary   string                 `json:"summary,omitempty"`
	ToolUseID string                 `json:"tool_use_id,omitempty"`
	Content   string                 `json:"content,omitempty"`
}

// UsageInfo represents token usage
type UsageInfo struct {
	InputTokens  int `json:"input_tokens,omitempty"`
	OutputTokens int `json:"output_tokens,omitempty"`
}

// Chat sends a message to Claude and returns the response
// Uses Claude's stream-json protocol with per-call spawning
func (a *Agent) Chat(message string) (*ChatResponse, error) {
	return a.ChatStream(message, nil)
}

// ChatStream sends a message and calls the callback for each content update
func (a *Agent) ChatStream(message string, callback StreamCallback) (*ChatResponse, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	// Build command args
	args := []string{
		"--dangerously-skip-permissions",
		"-p",
		"--verbose",
		"--output-format", "stream-json",
		"--input-format", "stream-json",
	}

	// Add streaming for real-time updates
	if callback != nil {
		args = append(args, "--include-partial-messages")
	}

	// Add system prompt on first call only (before session exists)
	if a.SessionID == "" && a.SystemPrompt != "" {
		args = append(args, "--system-prompt", a.SystemPrompt)
	}

	// Add MCP config if configured
	if a.MCPConfig != "" {
		args = append(args, "--mcp-config", a.MCPConfig)
	}

	// Add model flag if specified
	if a.Model != "" {
		args = append(args, "--model", a.Model)
	}

	// Add --resume if we have a session ID
	if a.SessionID != "" {
		args = append(args, "--resume", a.SessionID)
	}

	// Create the command
	cmd := a.spawner.commandCreator(a.spawner.claudePath, args...)
	cmd.Dir = a.WorkDir

	// Set up stdin with the message
	inputMsg := map[string]interface{}{
		"type": "user",
		"message": map[string]interface{}{
			"role":    "user",
			"content": message,
		},
	}
	inputBytes, err := json.Marshal(inputMsg)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal input: %w", err)
	}
	cmd.Stdin = bytes.NewReader(append(inputBytes, '\n'))

	// Set up stdout pipe for streaming
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	// Set up stderr pipe for capturing
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	// Start the command
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start claude: %w", err)
	}

	// Start goroutine to capture stderr
	var stderrBuf bytes.Buffer
	go func() {
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			line := scanner.Text()
			stderrBuf.WriteString(line + "\n")
			// Emit stderr to spawner
			if a.spawner != nil {
				a.spawner.emitOutput(a.ID, a.Name, line, "stderr")
			}
		}
	}()

	// Parse streaming output
	response := &ChatResponse{}
	var streamLines []string
	currentBlocks := map[int]*ChatContentBlock{}

	scanner := bufio.NewScanner(stdout)
	// Increase buffer size for large responses
	scanner.Buffer(make([]byte, 1024*1024), 10*1024*1024)

	for scanner.Scan() {
		line := scanner.Text()
		// Emit stdout to spawner
		if a.spawner != nil {
			a.spawner.emitOutput(a.ID, a.Name, line, "stdout")
		}
		if line == "" {
			continue
		}

		streamLines = append(streamLines, line)

		var msg StreamMessage
		if err := json.Unmarshal([]byte(line), &msg); err != nil {
			continue
		}

		if callback != nil {
			if block := updateStreamBlocksFromMessage(msg, currentBlocks); block != nil {
				callback(*block)
			}
		}

		// Capture session ID
		if msg.SessionID != "" && a.SessionID == "" {
			a.SessionID = msg.SessionID
			response.SessionID = msg.SessionID
		}

		// Handle final assistant message (non-streaming)
		if msg.Type == "assistant" && msg.Message != nil {
			response.Model = msg.Message.Model
			// If no streaming blocks, extract from final message
			if len(response.Blocks) == 0 {
				response.Blocks = append(response.Blocks, blocksFromAssistantMessage(*msg.Message)...)
			}

		}

		// Handle result message
		if msg.Type == "result" {
			if msg.IsError {
				return nil, fmt.Errorf("claude error: %s", msg.Result)
			}
			response.DurationMs = msg.DurationMs
			response.TotalCost = msg.TotalCostUSD
			if msg.Usage != nil {
				response.InputTokens = msg.Usage.InputTokens
				response.OutputTokens = msg.Usage.OutputTokens
			}
		}
	}

	for _, block := range parseStreamBlocks(streamLines) {
		response.Blocks = append(response.Blocks, block)
	}

	// Wait for command to finish
	if err := cmd.Wait(); err != nil {
		return nil, fmt.Errorf("claude command failed: %w (stderr: %s)", err, stderrBuf.String())
	}

	if len(response.Blocks) == 0 {
		return nil, fmt.Errorf("no response from claude (stderr: %s)", stderrBuf.String())
	}

	return response, nil
}

func blocksFromAssistantMessage(message ClaudeMessage) []ChatContentBlock {
	blocks := make([]ChatContentBlock, 0, len(message.Content))
	for _, cb := range message.Content {
		block := ChatContentBlock{}
		switch cb.Type {
		case "text":
			block.Type = ContentTypeText
			block.Text = cb.Text
		case "thinking":
			block.Type = ContentTypeThinking
			block.Text = cb.Text
			block.Summary = cb.Summary
		case "tool_use":
			block.Type = ContentTypeToolUse
			block.Name = cb.Name
			block.ID = cb.ID
			if cb.Input != nil {
				inputJSON, _ := json.Marshal(cb.Input)
				block.Input = string(inputJSON)
			}
		case "tool_result":
			block.Type = ContentTypeToolResult
			block.ID = cb.ToolUseID
			block.Text = cb.Content
		}
		blocks = append(blocks, block)
	}
	return blocks
}

// Send sends a message (alias for Chat, for compatibility)
func (a *Agent) Send(method string, params interface{}) error {
	if p, ok := params.(map[string]interface{}); ok {
		if msg, ok := p["message"].(string); ok {
			_, err := a.Chat(msg)
			return err
		}
	}
	return fmt.Errorf("invalid params for Send")
}

// IsRunning returns true if the agent is available for messages
func (a *Agent) IsRunning() bool {
	return a.spawner != nil
}

// Kill clears the session (no persistent process to kill)
func (a *Agent) Kill() error {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.SessionID = ""
	return nil
}

// GetTextFromBlocks extracts text from ContentBlocks (legacy helper)
func GetTextFromBlocks(blocks []ContentBlock) string {
	var parts []string
	for _, b := range blocks {
		if b.Type == "text" {
			parts = append(parts, b.Text)
		}
	}
	return strings.Join(parts, "")
}

// Close implements io.Closer for stdout pipe
func closeReader(r io.Reader) {
	if closer, ok := r.(io.Closer); ok {
		closer.Close()
	}
}
