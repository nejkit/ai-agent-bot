package provider

import (
	"errors"
	"fmt"
	"github.com/sirupsen/logrus"
	"io"
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

	logrus.Infof("Link to file: %s", fmt.Sprintf("https://api.telegram.org/file/bot%s/%s", t.api.Token, fileInfo.FilePath))

	httpResponse, err := http.Get(fmt.Sprintf("https://api.telegram.org/file/bot%s/%s", t.api.Token, fileInfo.FilePath))

	if err != nil {
		return nil, err
	}

	body, err := io.ReadAll(httpResponse.Body)

	if err != nil {
		return nil, err
	}

	return body, nil
}

func (t *TelegramClient) GetChatInfoByID(cfg tgbotapi.ChatConfig) (*tgbotapi.Chat, error) {
	chat, err := t.api.GetChat(tgbotapi.ChatInfoConfig{ChatConfig: cfg})

	if err != nil {
		return nil, err
	}

	return &chat, nil
}

func (t *TelegramClient) GetChatInfoByUserName(superGroupName string) (*tgbotapi.Chat, error) {
	chatCfg := tgbotapi.ChatConfig{SuperGroupUsername: superGroupName}

	chat, err := t.api.GetChat(tgbotapi.ChatInfoConfig{ChatConfig: chatCfg})

	if err != nil {
		return nil, err
	}

	return &chat, nil
}

func (t *TelegramClient) GetChatOwnerId(chatID int64) (int64, error) {
	chatCfg := tgbotapi.ChatConfig{ChatID: chatID}

	chatMembers, err := t.api.GetChatAdministrators(tgbotapi.ChatAdministratorsConfig{ChatConfig: chatCfg})

	if err != nil {
		return 0, err
	}

	for index := range chatMembers {
		if chatMembers[index].IsCreator() {
			return chatMembers[index].User.ID, nil
		}
	}

	return 0, errors.New("not found chat creator")
}

func (t *TelegramClient) GetChatOwnerInfo(chatID int64) (*tgbotapi.User, error) {
	chatCfg := tgbotapi.ChatConfig{ChatID: chatID}

	chatMembers, err := t.api.GetChatAdministrators(tgbotapi.ChatAdministratorsConfig{ChatConfig: chatCfg})

	if err != nil {
		return nil, err
	}

	for index := range chatMembers {
		if chatMembers[index].IsCreator() {
			return chatMembers[index].User, nil
		}
	}

	return nil, errors.New("not found chat creator")
}
