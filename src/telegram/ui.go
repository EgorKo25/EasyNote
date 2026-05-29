package telegram

import (
	"fmt"
	"time"

	"github.com/EgorKo25/EasyNote/src/database"
	"github.com/mymmrac/telego"
	tu "github.com/mymmrac/telego/telegoutil"
)

const (
	buttonStart          = "Старт"
	buttonCreateReminder = "Создать напоминание"
	buttonReminderList   = "Список напоминаний"
)
const (
	callbackDraftTitle       = "draft:title"
	callbackDraftDescription = "draft:description"
	callbackDraftTime        = "draft:time"
	callbackDraftSave        = "draft:save"
	callbackDraftCancel      = "draft:cancel"
	callbackDeletePrefix     = "reminder:delete:"
)
const (
	fieldTitle       draftField = "title"
	fieldDescription draftField = "description"
	fieldTime        draftField = "time"
)

const emptyFieldText = "не задано"

func mainKeyboard() *telego.ReplyKeyboardMarkup {
	return tu.Keyboard(
		tu.KeyboardRow(tu.KeyboardButton(buttonCreateReminder)),
		tu.KeyboardRow(tu.KeyboardButton(buttonReminderList)),
	).WithResizeKeyboard()
}

func draftKeyboard() *telego.InlineKeyboardMarkup {
	return tu.InlineKeyboard(
		tu.InlineKeyboardRow(tu.InlineKeyboardButton("Добавить время").WithCallbackData(callbackDraftTime)),
		tu.InlineKeyboardRow(tu.InlineKeyboardButton("Добавить название").WithCallbackData(callbackDraftTitle)),
		tu.InlineKeyboardRow(tu.InlineKeyboardButton("Добавить описание").WithCallbackData(callbackDraftDescription)),
		tu.InlineKeyboardRow(tu.InlineKeyboardButton("Сохранить").WithCallbackData(callbackDraftSave)),
		tu.InlineKeyboardRow(tu.InlineKeyboardButton("Отмена").WithCallbackData(callbackDraftCancel)),
	)
}

func deleteKeyboard(reminderID int64) *telego.InlineKeyboardMarkup {
	return tu.InlineKeyboard(
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton("Удалить").
				WithCallbackData(fmt.Sprintf("%s%d", callbackDeletePrefix, reminderID)),
		),
	)
}

func draftText(draft reminderDraft) string {
	remindAt, title, description := emptyFieldText, emptyFieldText, emptyFieldText

	if !draft.remindAt.IsZero() {
		remindAt = draft.remindAt.Format(time.DateTime)
	}

	if draft.title != "" {
		title = draft.title
	}

	if draft.description != "" {
		description = draft.description
	}

	return fmt.Sprintf(
		"Напоминание\nВремя напоминания: %s\nНазвание: %s\nОписание: %s",
		remindAt,
		title,
		description,
	)
}

func reminderViewText(reminder database.ReminderView) string {
	description := "не задано"
	if reminder.Description != "" {
		description = reminder.Description
	}

	return fmt.Sprintf(
		"Напоминание #%d\nВремя: %s\nНазвание: %s\nОписание: %s",
		reminder.ID,
		reminder.RemindAt.Format(time.DateTime),
		reminder.Title,
		description,
	)
}
