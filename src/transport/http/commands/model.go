package commands

import (
	"time"

	"github.com/EgorKo25/EasyNote/src/database"
)

type ReminderDTO struct {
	ID          int64                   `json:"id"`
	RuleID      *int64                  `json:"rule_id"`
	Title       string                  `json:"title"`
	Description string                  `json:"description"`
	RemindAt    time.Time               `json:"remind_at"`
	Status      database.ReminderStatus `json:"status"`
	CreatedAt   time.Time               `json:"created_at"`
}
