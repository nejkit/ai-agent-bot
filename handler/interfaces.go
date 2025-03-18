package handler

import (
	"context"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/nejkit/ai-agent-bot/models"
)

type telegramClient interface {
	SendReplyMessageForChatId(chatId int64, messageToReply int, text string) (int, error)
	GetChatOwnerId(chatID int64) (int64, error)
	GetChatInfoByID(cfg tgbotapi.ChatConfig) (*tgbotapi.Chat, error)
	GetChatOwnerInfo(chatID int64) (*tgbotapi.User, error)
}

type messagesProvider interface {
	SaveChatToAllowed(chatId int64) error
	GetAllowedChats() []int64
	DeleteChatFromAllowed(chatId int64) error
	GetSettingsForSuperGroupChat(chatId int64) (*models.SuperGroupConfigModel, error)
	SaveSettingsForSuperGroupChat(chatId int64, info *models.SuperGroupConfigModel) error
}

type ticketProvider interface {
	SaveTicket(data *models.ExternalChatTicketData) error
	StoreTicketIntoPool(chatId int64, ticketId string, requestMessageId int) error
}

type chatContainer interface {
	RemoveChatManager(chatID int64) error
	AddChatManager(rootCtx context.Context, chatID int64) error
}
