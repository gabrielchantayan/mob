package tui

type Toast struct {
	Message string
}

type ToastQueue struct {
	items []Toast
}

func NewToastQueue() *ToastQueue {
	return &ToastQueue{}
}

func (queue *ToastQueue) Push(toast Toast) {
	queue.items = append(queue.items, toast)
}

func (queue *ToastQueue) Peek() (Toast, bool) {
	if len(queue.items) == 0 {
		return Toast{}, false
	}
	return queue.items[0], true
}

func (queue *ToastQueue) Pop() (Toast, bool) {
	if len(queue.items) == 0 {
		return Toast{}, false
	}
	item := queue.items[0]
	queue.items = queue.items[1:]
	return item, true
}

func (queue *ToastQueue) Len() int {
	return len(queue.items)
}
