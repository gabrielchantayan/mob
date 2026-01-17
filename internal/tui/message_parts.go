package tui

import (
	"strings"

	"github.com/gabe/mob/internal/agent"
)

type partType string

const (
	partReasoning partType = "reasoning"
	partTool      partType = "tool"
	partText      partType = "text"
)

type chatPart struct {
	Type       partType
	Text       string
	Summary    string
	ToolName   string
	ToolInput  string
	ToolOutput string
	ToolID     string
}

func buildAssistantParts(blocks []agent.ChatContentBlock) []chatPart {
	var parts []chatPart
	toolIndex := map[string]int{}

	for _, block := range blocks {
		switch block.Type {
		case agent.ContentTypeThinking:
			parts = append(parts, chatPart{Type: partReasoning, Text: block.Text, Summary: block.Summary})
		case agent.ContentTypeToolUse:
			toolIndex[block.ID] = len(parts)
			parts = append(parts, chatPart{Type: partTool, ToolName: block.Name, ToolInput: block.Input, ToolID: block.ID})
		case agent.ContentTypeToolResult:
			if idx, ok := toolIndex[block.ID]; ok {
				parts[idx].ToolOutput = block.Text
			} else {
				parts = append(parts, chatPart{Type: partTool, ToolName: "unknown", ToolOutput: block.Text, ToolID: block.ID})
			}
		case agent.ContentTypeText:
			if strings.TrimSpace(block.Text) != "" {
				parts = append(parts, chatPart{Type: partText, Text: block.Text})
			}
		}
	}

	return parts
}
