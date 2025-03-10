package provider

import (
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
)

type TelegramClient struct {
	api *tgbotapi.BotAPI
}

func NewTelegramClient(token string) (*TelegramClient, error) {
	api, err := tgbotapi.NewBotAPI(token)

	if err != nil {
		return nil, err
	}

	return &TelegramClient{api: api}, nil
}

func (t *TelegramClient) SendReplyMessageForChatId(chatId int64, messageToReply int, text string) (int, error) {
	msgConfig := tgbotapi.NewMessage(chatId, text)

	msgConfig.ReplyToMessageID = messageToReply

	response, err := t.api.Send(msgConfig)

	if err != nil {
		return 0, err
	}

	return response.MessageID, nil
}

func (t *TelegramClient) EditReplyMessageForChatId(chatId int64, messageId int, text string) error {
	msgConfig := tgbotapi.NewEditMessageText(chatId, messageId, text)

	_, err := t.api.Send(msgConfig)

	return err
}
