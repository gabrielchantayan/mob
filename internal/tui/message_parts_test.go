package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/gabe/mob/internal/agent"
	"github.com/muesli/termenv"
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

func TestRenderAssistantFooter(t *testing.T) {
	lipgloss.SetColorProfile(termenv.Ascii)
	m := Model{}
	msg := ChatMessage{Model: "claude-3-5-sonnet", DurationMs: 1200}
	out := m.renderAssistantFooter(msg, 60)
	if !strings.Contains(out, "▣ Build") || !strings.Contains(out, "sonnet") {
		t.Fatalf("footer missing expected tokens: %s", out)
	}
}

func TestRenderAssistantToolOutput(t *testing.T) {
	lipgloss.SetColorProfile(termenv.Ascii)
	m := Model{}
	msg := ChatMessage{Blocks: []agent.ChatContentBlock{
		{Type: agent.ContentTypeToolUse, Name: "bash", Input: `{"command":"ls"}`, ID: "call-1"},
		{Type: agent.ContentTypeToolResult, ID: "call-1", Text: "file.txt"},
		{Type: agent.ContentTypeText, Text: "done"},
	}}

	out := m.renderAssistantMessage(msg, 60)
	if !strings.Contains(out, "file.txt") {
		t.Fatalf("expected tool output to render, got: %s", out)
	}
}

func TestRenderToolOutputPreservesWhitespace(t *testing.T) {
	lipgloss.SetColorProfile(termenv.Ascii)
	m := Model{}
	msg := ChatMessage{Blocks: []agent.ChatContentBlock{
		{Type: agent.ContentTypeToolUse, Name: "bash", Input: `{"command":"ls"}`, ID: "call-1"},
		{Type: agent.ContentTypeToolResult, ID: "call-1", Text: "col1  col2"},
	}}

	out := m.renderAssistantMessage(msg, 60)
	if !strings.Contains(out, "col1  col2") {
		t.Fatalf("expected whitespace preserved, got: %s", out)
	}
}

func TestRenderContentBlockUsesParts(t *testing.T) {
	lipgloss.SetColorProfile(termenv.Ascii)
	m := Model{}
	block := agent.ChatContentBlock{Type: agent.ContentTypeText, Text: "hello"}
	out := m.renderContentBlock(block, 40)
	if !strings.Contains(out, "hello") {
		t.Fatalf("expected content block text, got: %s", out)
	}
}
