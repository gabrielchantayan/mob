package agent

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

func TestParseToolResultBlock(t *testing.T) {
	lines := []string{
		`{"type":"stream_event","event":{"type":"content_block_start","index":1,"content_block":{"type":"tool_result","tool_use_id":"call-1","content":"ok"}}}`,
		`{"type":"stream_event","event":{"type":"content_block_stop","index":1}}`,
	}

	blocks := parseStreamBlocks(lines)
	if len(blocks) != 1 {
		t.Fatalf("expected 1 block")
	}
	if blocks[0].Type != ContentTypeToolResult || blocks[0].ID != "call-1" || blocks[0].Text != "ok" {
		t.Fatalf("unexpected tool_result block: %+v", blocks[0])
	}
}

func TestParseStreamBlocksDelta(t *testing.T) {
	lines := []string{
		`{"type":"stream_event","event":{"type":"content_block_start","index":0,"content_block":{"type":"text"}}}`,
		`{"type":"stream_event","event":{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"hello "}}}`,
		`{"type":"stream_event","event":{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"world"}}}`,
		`{"type":"stream_event","event":{"type":"content_block_stop","index":0}}`,
	}

	blocks := parseStreamBlocks(lines)
	if len(blocks) != 1 {
		t.Fatalf("expected 1 block, got %d", len(blocks))
	}
	if blocks[0].Type != ContentTypeText || blocks[0].Text != "hello world" {
		t.Fatalf("expected type=%s text=%q, got type=%s text=%q", ContentTypeText, "hello world", blocks[0].Type, blocks[0].Text)
	}
}

func TestChatStreamEmitsCallbacks(t *testing.T) {
	lines := []string{
		`{"type":"stream_event","event":{"type":"content_block_start","index":0,"content_block":{"type":"text"}}}`,
		`{"type":"stream_event","event":{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"hello "}}}`,
		`{"type":"stream_event","event":{"type":"content_block_start","index":1,"content_block":{"type":"tool_result","tool_use_id":"call-1","content":"ok"}}}`,
		`{"type":"stream_event","event":{"type":"content_block_stop","index":1}}`,
		`{"type":"stream_event","event":{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"world"}}}`,
		`{"type":"stream_event","event":{"type":"content_block_stop","index":0}}`,
	}

	callbackBlocks, responseBlocks := runChatStreamLines(lines)

	if len(callbackBlocks) == 0 {
		t.Fatalf("expected callbacks to fire")
	}
	if len(responseBlocks) != 2 {
		t.Fatalf("expected 2 response blocks, got %d", len(responseBlocks))
	}
	if len(callbackBlocks) <= len(responseBlocks) {
		t.Fatalf("expected incremental callbacks, got %d callbacks for %d response blocks", len(callbackBlocks), len(responseBlocks))
	}

	var sawPartialText bool
	var sawToolResult bool
	for _, block := range callbackBlocks {
		if block.Type == ContentTypeText && block.Text == "hello " {
			sawPartialText = true
		}
		if block.Type == ContentTypeToolResult && block.ID == "call-1" {
			sawToolResult = true
		}
	}
	if !sawPartialText {
		t.Fatalf("expected partial text callback for hello ")
	}
	if !sawToolResult {
		t.Fatalf("expected tool_result callback for call-1")
	}
	if callbackBlocks[len(callbackBlocks)-1].Type != ContentTypeText || callbackBlocks[len(callbackBlocks)-1].Text != "hello world" {
		t.Fatalf("expected final text callback, got %+v", callbackBlocks[len(callbackBlocks)-1])
	}
	if responseBlocks[0].Type != ContentTypeToolResult || responseBlocks[0].Text != "ok" {
		t.Fatalf("expected tool_result response block, got %+v", responseBlocks[0])
	}
	if responseBlocks[1].Type != ContentTypeText || responseBlocks[1].Text != "hello world" {
		t.Fatalf("expected text response block, got %+v", responseBlocks[1])
	}
}

func runChatStreamLines(lines []string) ([]ChatContentBlock, []ChatContentBlock) {
	temporaryDir := newTempDir()
	commandArgs := []string{"-test.run=TestHelperStreamProcess", "--", strings.Join(lines, "\n")}
	cmd := exec.Command(os.Args[0], commandArgs...)
	cmd.Env = append(os.Environ(), "GO_WANT_HELPER_PROCESS=1")
	cmd.Dir = temporaryDir

	spawner := NewSpawner()
	spawner.SetCommandCreator(func(name string, args ...string) *exec.Cmd {
		return cmd
	})

	agent := &Agent{
		ID:      "agent-1",
		Name:    "agent",
		WorkDir: temporaryDir,
		spawner: spawner,
	}

	var callbacks []ChatContentBlock
	response, err := agent.ChatStream("hi", func(block ChatContentBlock) {
		callbacks = append(callbacks, block)
	})
	if err != nil {
		return callbacks, nil
	}

	return callbacks, response.Blocks
}

func TestAssistantMessageToolResult(t *testing.T) {
	blocks := blocksFromAssistantMessage(ClaudeMessage{Content: []ContentBlock{{
		Type:      "tool_result",
		ToolUseID: "call-1",
		Content:   "ok",
	}}})

	if len(blocks) != 1 {
		t.Fatalf("expected 1 block, got %d", len(blocks))
	}
	if blocks[0].Type != ContentTypeToolResult || blocks[0].ID != "call-1" || blocks[0].Text != "ok" {
		t.Fatalf("unexpected tool_result mapping: %+v", blocks[0])
	}
}

func newTempDir() string {
	dir, _ := os.MkdirTemp("", "mob-agent-test")
	return dir
}

func TestHelperStreamProcess(t *testing.T) {
	if os.Getenv("GO_WANT_HELPER_PROCESS") != "1" {
		return
	}
	if len(os.Args) < 4 || os.Args[2] != "--" {
		os.Exit(1)
	}

	for _, line := range strings.Split(os.Args[3], "\n") {
		fmt.Fprintln(os.Stdout, line)
		time.Sleep(2 * time.Millisecond)
	}
	os.Exit(0)
}
