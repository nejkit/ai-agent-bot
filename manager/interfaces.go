package manager

import (
	"context"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/nejkit/ai-agent-bot/models"
	"github.com/nejkit/ai-agent-bot/storage"
)

type telegramClient interface {
	EditReplyMessageForChatId(chatId int64, messageId int, text string) error
	DownloadFileById(fileId string) ([]byte, error)
	GetChatOwnerId(chatID int64) (int64, error)
	GetChatInfoByID(config tgbotapi.ChatConfig) (*tgbotapi.Chat, error)
	GetChatInfoByUserName(superGroupName string) (*tgbotapi.Chat, error)
	GetChatOwnerInfo(chatID int64) (*tgbotapi.User, error)
	SendMessageWithFile(chatId int64, message string, file []byte) error
	SendMessage(chatId int64, message string) error
}

type messagesProvider interface {
	GetMessagesForChatId(chatId string) ([]models.MessageData, error)
	SaveMessagesForChatId(chatId string, messages []models.MessageData) error
	GetAllowedChats() []int64
	DeleteChatFromAllowed(chatId int64) error
	GetSettingsForSuperGroupChat(chatId int64) (*models.SuperGroupConfigModel, error)
	SaveSettingsForSuperGroupChat(chatId int64, info *models.SuperGroupConfigModel) error
	SaveChatToAllowed(chatId int64) error

	SaveReportMeta(hash string, author string, formDate string) error
	GetReportMetadata(hash string) (*storage.ReportMeta, error)
}

type ticketProvider interface {
	GetTicketById(ticketId string) (*models.ExternalChatTicketData, error)
	SaveTicket(data *models.ExternalChatTicketData) error
	DeleteTicket(ticketId string) error
	GetTicketFromPool(chatId int64) (string, error)
	DeleteTicketFromPool(chatId int64, ticketId string) error
	StoreTicketIntoPool(chatId int64, ticketId string, requestMessageId int) error
}

type openAICli interface {
	SendMessagesToAI(ctx context.Context, messages []models.MessageData) (string, error)
	UploadFile(ctx context.Context, content []byte) (string, error)
	SendMessageWithFileToAI(ctx context.Context, messages []models.MessageData, assistantId string, fileId string) (string, string, error)
	PollResponseFromAssistant(ctx context.Context, threadId string, runId string) (string, string, error)
	DownloadFile(ctx context.Context, fileId string) ([]byte, error)
}
