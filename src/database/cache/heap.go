package cache

type reminderHeap []*ScheduledReminder

func (h reminderHeap) Len() int {
	return len(h)
}

func (h reminderHeap) Less(i, j int) bool {
	return h[i].RemindAt.Before(h[j].RemindAt)
}

func (h reminderHeap) Swap(i, j int) {
	h[i], h[j] = h[j], h[i]

	h[i].index = i
	h[j].index = j
}

func (h *reminderHeap) Push(value any) {
	item := value.(*ScheduledReminder)
	item.index = len(*h)

	*h = append(*h, item)
}

func (h *reminderHeap) Pop() any {
	old := *h
	n := len(old)

	item := old[n-1]
	item.index = -1

	old[n-1] = nil
	*h = old[:n-1]

	return item
}
