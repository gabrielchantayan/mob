package agent

import "encoding/json"

func updateStreamBlocksFromMessage(msg StreamMessage, current map[int]*ChatContentBlock) *ChatContentBlock {
	if msg.Type != "stream_event" || msg.Event == nil {
		return nil
	}

	switch msg.Event.Type {
	case "content_block_start":
		if msg.Event.ContentBlock == nil {
			return nil
		}
		block := &ChatContentBlock{Index: msg.Event.Index}
		switch msg.Event.ContentBlock.Type {
		case "tool_result":
			block.Type = ContentTypeToolResult
			block.ID = msg.Event.ContentBlock.ToolUseID
			block.Text = msg.Event.ContentBlock.Content
		case "tool_use":
			block.Type = ContentTypeToolUse
			block.Name = msg.Event.ContentBlock.Name
			block.ID = msg.Event.ContentBlock.ID
		case "thinking":
			block.Type = ContentTypeThinking
			block.Summary = msg.Event.ContentBlock.Summary
		case "text":
			block.Type = ContentTypeText
		}
		current[msg.Event.Index] = block
		copy := *block
		return &copy
	case "content_block_delta":
		if block := current[msg.Event.Index]; block != nil && msg.Event.Delta != nil {
			switch msg.Event.Delta.Type {
			case "text_delta", "thinking_delta":
				block.Text += msg.Event.Delta.Text
			case "summary_delta":
				block.Summary += msg.Event.Delta.Summary
			case "input_json_delta":
				block.Input += msg.Event.Delta.Text
			}
			copy := *block
			return &copy
		}
	case "content_block_stop":
		if block := current[msg.Event.Index]; block != nil {
			delete(current, msg.Event.Index)
			copy := *block
			return &copy
		}
	}

	return nil
}

func parseStreamBlocks(lines []string) []ChatContentBlock {
	current := map[int]*ChatContentBlock{}
	var blocks []ChatContentBlock

	for _, line := range lines {
		var msg StreamMessage
		if err := json.Unmarshal([]byte(line), &msg); err != nil {
			continue
		}
		block := updateStreamBlocksFromMessage(msg, current)
		if msg.Type != "stream_event" || msg.Event == nil || msg.Event.Type != "content_block_stop" {
			continue
		}
		if block != nil {
			blocks = append(blocks, *block)
		}
	}

	return blocks
}
