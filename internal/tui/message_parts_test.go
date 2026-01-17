package tui

import (
	"strings"
	"testing"

	"github.com/gabe/mob/internal/agent"
)

func TestBuildAssistantPartsOrders(t *testing.T) {
	blocks := []agent.ChatContentBlock{
		{Type: agent.ContentTypeThinking, Text: "plan", Summary: ""},
		{Type: agent.ContentTypeToolUse, Name: "bash", Input: `{"command":"ls"}`, ID: "call-1"},
		{Type: agent.ContentTypeToolResult, ID: "call-1", Text: "file.txt"},
		{Type: agent.ContentTypeText, Text: "done"},
	}

	parts := buildAssistantParts(blocks)

	if len(parts) != 3 {
		t.Fatalf("expected 3 parts, got %d", len(parts))
	}
	if parts[0].Type != partReasoning || !strings.Contains(parts[0].Text, "plan") {
		t.Fatalf("expected reasoning part first")
	}
	if parts[1].Type != partTool || parts[1].ToolOutput == "" {
		t.Fatalf("expected tool output attached")
	}
	if parts[2].Type != partText || !strings.Contains(parts[2].Text, "done") {
		t.Fatalf("expected text part last")
	}
}
