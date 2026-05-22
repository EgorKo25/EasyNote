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

	h[i].Index = i
	h[j].Index = j
}

func (h *reminderHeap) Push(value any) {
	item, _ := value.(*ScheduledReminder)
	item.Index = len(*h)

	*h = append(*h, item)
}

func (h *reminderHeap) Pop() any {
	old := *h
	lengthOldHeap := len(old)

	item := old[lengthOldHeap-1]
	item.Index = -1

	old[lengthOldHeap-1] = nil
	*h = old[:lengthOldHeap-1]

	return item
}
