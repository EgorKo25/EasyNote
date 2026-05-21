package cache

import (
	"container/heap"
	"github.com/EgorKo25/EasyNote/src/database"
	"time"
)

func NewScheduledReminder(note database.Note, reminder database.Reminder) ScheduledReminder {
	return ScheduledReminder{
		ID:             reminder.ID,
		NoteID:         reminder.NoteID,
		TelegramChatID: note.TelegramChatID,
		Text:           note.Text,
		RemindAt:       reminder.RemindAt,
	}
}

type ScheduledReminder struct {
	ID int64

	NoteID int64

	TelegramChatID int64
	Text           string

	RemindAt time.Time

	index int
}

func NewReminderCache(cacheWindow time.Duration) *ReminderCache {
	items := make(reminderHeap, 0)
	heap.Init(&items)

	return &ReminderCache{
		cacheWindow: cacheWindow,
		heap:        items,
		byID:        make(map[int64]*ScheduledReminder),
	}
}

type ReminderCache struct {
	cacheWindow time.Duration

	heap reminderHeap
	byID map[int64]*ScheduledReminder
}

func (c *ReminderCache) AddIfWithinWindow(now time.Time, reminder ScheduledReminder) bool {
	if reminder.RemindAt.After(now.Add(c.cacheWindow)) {
		return false
	}

	c.Upsert(reminder)

	return true
}

func (c *ReminderCache) Upsert(reminder ScheduledReminder) {
	if item, ok := c.byID[reminder.ID]; ok {
		item.NoteID = reminder.NoteID
		item.TelegramChatID = reminder.TelegramChatID
		item.Text = reminder.Text
		item.RemindAt = reminder.RemindAt

		heap.Fix(&c.heap, item.index)

		return
	}

	item := &ScheduledReminder{
		ID:             reminder.ID,
		NoteID:         reminder.NoteID,
		TelegramChatID: reminder.TelegramChatID,
		Text:           reminder.Text,
		RemindAt:       reminder.RemindAt,
	}

	heap.Push(&c.heap, item)
	c.byID[item.ID] = item
}

func (c *ReminderCache) Next() (ScheduledReminder, bool) {
	if c.heap.Len() == 0 {
		return ScheduledReminder{}, false
	}

	return *c.heap[0], true
}

func (c *ReminderCache) PopDue(now time.Time) []ScheduledReminder {
	due := make([]ScheduledReminder, 0)

	for c.heap.Len() > 0 {
		next := c.heap[0]
		if next.RemindAt.After(now) {
			break
		}

		item := heap.Pop(&c.heap).(*ScheduledReminder)
		delete(c.byID, item.ID)

		due = append(due, *item)
	}

	return due
}

func (c *ReminderCache) Remove(id int64) bool {
	item, ok := c.byID[id]
	if !ok {
		return false
	}

	heap.Remove(&c.heap, item.index)
	delete(c.byID, id)

	return true
}

func (c *ReminderCache) Exists(id int64) bool {
	_, ok := c.byID[id]

	return ok
}

func (c *ReminderCache) Len() int {
	return c.heap.Len()
}

func (c *ReminderCache) Window() time.Duration {
	return c.cacheWindow
}
