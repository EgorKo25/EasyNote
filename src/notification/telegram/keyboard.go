package telegram

import (
	"github.com/mymmrac/telego"
	tu "github.com/mymmrac/telego/telegoutil"
)

const (
	notificationButtonCreateReminder = "Создать"
	notificationButtonReminders      = "Напоминания"
	notificationButtonShoppingLists  = "🛒 Списки"
	notificationButtonHolidays       = "🎉 Праздники"
	notificationButtonSettings       = "Настройки"
)

func notificationMainKeyboard() *telego.ReplyKeyboardMarkup {
	return tu.Keyboard(
		tu.KeyboardRow(tu.KeyboardButton("➕ "+notificationButtonCreateReminder)),
		tu.KeyboardRow(
			tu.KeyboardButton("⏰ "+notificationButtonReminders),
			tu.KeyboardButton(notificationButtonShoppingLists),
		),
		tu.KeyboardRow(
			tu.KeyboardButton(notificationButtonHolidays),
			tu.KeyboardButton("⚙️ "+notificationButtonSettings),
		),
	).WithResizeKeyboard()
}
