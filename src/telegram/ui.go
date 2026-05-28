package telegram

import (
	"fmt"
	"time"

	"github.com/EgorKo25/EasyNote/src/database"
	"github.com/mymmrac/telego"
	tu "github.com/mymmrac/telego/telegoutil"
)

const buttonStart = "Старт"
const buttonCreateReminder = "Создать напоминание"
const buttonReminderList = "Список напоминаний"

const callbackDraftTitle = "draft:title"
const callbackDraftDescription = "draft:description"
const callbackDraftTime = "draft:time"
const callbackDraftSave = "draft:save"
const callbackDraftCancel = "draft:cancel"
const callbackDeletePrefix = "reminder:delete:"

const fieldTitle draftField = "title"
const fieldDescription draftField = "description"
const fieldTime draftField = "time"

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
	remindAt := "не задано"
	if !draft.remindAt.IsZero() {
		remindAt = draft.remindAt.Format(time.DateTime)
	}

	title := "не задано"
	if draft.title != "" {
		title = draft.title
	}

	description := "не задано"
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
