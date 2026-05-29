package cache

import (
	"container/heap"
	"time"

	"github.com/EgorKo25/EasyNote/src/database"
)

type ScheduledReminder = database.ScheduledReminder

func NewScheduledReminder(note database.Note, reminder database.Reminder) ScheduledReminder {
	return ScheduledReminder{
		ID:             reminder.ID,
		NoteID:         reminder.NoteID,
		TelegramChatID: note.TelegramChatID,
		Title:          note.Title,
		Description:    note.Description,
		RemindAt:       reminder.RemindAt,
	}
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
		item.Title = reminder.Title
		item.Description = reminder.Description
		item.RemindAt = reminder.RemindAt

		heap.Fix(&c.heap, item.Index)

		return
	}

	item := &ScheduledReminder{
		ID:             reminder.ID,
		NoteID:         reminder.NoteID,
		TelegramChatID: reminder.TelegramChatID,
		Title:          reminder.Title,
		Description:    reminder.Description,
		RemindAt:       reminder.RemindAt,
	}

	heap.Push(&c.heap, item)
	c.byID[item.ID] = item
}

func (c *ReminderCache) Replace(reminders []ScheduledReminder) {
	c.heap = make(reminderHeap, 0, len(reminders))
	c.byID = make(map[int64]*ScheduledReminder, len(reminders))
	heap.Init(&c.heap)

	for _, reminder := range reminders {
		c.Upsert(reminder)
	}
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

		item, ok := heap.Pop(&c.heap).(*ScheduledReminder)
		if !ok {
			continue
		}

		delete(c.byID, item.ID)

		due = append(due, *item)
	}

	return due
}

func (c *ReminderCache) Remove(elementID int64) bool {
	item, ok := c.byID[elementID]
	if !ok {
		return false
	}

	heap.Remove(&c.heap, item.Index)
	delete(c.byID, elementID)

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
