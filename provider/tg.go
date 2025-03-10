package provider

import (
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
)

type TelegramClient struct {
	api *tgbotapi.BotAPI
}
