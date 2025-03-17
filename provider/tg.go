package provider

import (
	"fmt"
	"net/http"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type TelegramClient struct {
	api *tgbotapi.BotAPI
}

func NewTelegramClient(api *tgbotapi.BotAPI) *TelegramClient {
	return &TelegramClient{api: api}
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

func (t *TelegramClient) DownloadFileById(fileId string) ([]byte, error) {
	fileConfig := tgbotapi.FileConfig{FileID: fileId}

	fileInfo, err := t.api.GetFile(fileConfig)

	if err != nil {
		return nil, err
	}

	response, err := http.Get(fmt.Sprintf("https://api.telegram.org/file/bot%s/%s", t.api.Token, fileInfo.FilePath))

	if err != nil {
		return nil, err
	}

	fileBytes := []byte{}

	_, err = response.Body.Read(fileBytes)

	if err != nil {
		return nil, err
	}

	return fileBytes, nil
}
