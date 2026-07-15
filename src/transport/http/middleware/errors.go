package middleware

import "errors"

var ErrTelegramChatIDMissing = errors.New("telegram chat id is missing")
var ErrTelegramChatIDInvalid = errors.New("telegram chat id is invalid")
