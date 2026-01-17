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

func (queue *ToastQueue) Len() int {
	return len(queue.items)
}
