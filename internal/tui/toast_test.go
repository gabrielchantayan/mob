package tui

import "testing"

func TestToastQueue(t *testing.T) {
	queue := NewToastQueue()
	queue.Push(Toast{Message: "hi"})
	if queue.Len() != 1 {
		t.Fatal("expected toast")
	}
}
