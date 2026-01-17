package tui

import "testing"

func TestToastQueue(t *testing.T) {
	queue := NewToastQueue()
	queue.Push(Toast{Message: "first"})
	queue.Push(Toast{Message: "second"})
	if queue.Len() != 2 {
		t.Fatal("expected toast")
	}
	peek, ok := queue.Peek()
	if !ok || peek.Message != "first" {
		t.Fatal("expected first toast")
	}
	popped, ok := queue.Pop()
	if !ok || popped.Message != "first" {
		t.Fatal("expected pop first toast")
	}
	if queue.Len() != 1 {
		t.Fatal("expected one toast")
	}
	popped, ok = queue.Pop()
	if !ok || popped.Message != "second" {
		t.Fatal("expected pop second toast")
	}
	_, ok = queue.Pop()
	if ok {
		t.Fatal("expected empty pop")
	}
}
