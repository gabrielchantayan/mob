package agent

import "testing"

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
