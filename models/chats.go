package models

import (
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/google/uuid"
)

type MessageData struct {
	Text      string
	CreatedBy MessageType
}

type MessageType int

const (
	MessageTypeUser = iota
	MessageTypeAssistant
)

func BuildTicketWithText(chatId int64, chatCtxKey string, message *tgbotapi.Message) *ExternalChatTicketData {
	fileId := ""
	text := "Проаналізуй файл"

	if message.Text != "" {
		text = message.Text
	}

	if message.Document != nil {
		fileId = message.Document.FileID
	}

	if message.Photo != nil {
		fileId = message.Photo[0].FileID
	}

	return &ExternalChatTicketData{
		Id:             uuid.NewString(),
		ChatId:         chatId,
		ChatContextKey: chatCtxKey,
		Status:         TicketStatusNew,
		Action:         TicketActionValidation,
		Type:           TicketTypeMessaging,
		ChatContext:    make([]MessageData, 0),
		Request: &RequestData{
			Text:      text,
			FileId:    fileId,
			MessageId: message.MessageID,
		},
		Response:   &ResponseData{},
		RetryAt:    0,
		RetryCount: 0,
	}
}

type ExternalChatTicketData struct {
	Id             string
	ChatId         int64
	ChatContextKey string

	Status TicketStatus
	Action TicketAction
	Type   TicketType

	ChatContext []MessageData

	Request  *RequestData
	Response *ResponseData

	AssistantData *AssistantData

	Error error

	RetryAt    int64
	RetryCount int
}

type RequestData struct {
	Text      string
	FileId    string
	MessageId int
}

type ResponseData struct {
	Text        string
	FileContent []byte
	MessageId   int
}

type AssistantData struct {
	ThreadId string
	RunId    string
	FileId   string
}

type TicketStatus int
type TicketAction int
type TicketType int

const (
	TicketStatusNew TicketStatus = iota
	TicketStatusInProgress
	TicketStatusWaitResponse
	TicketStatusError
)

const (
	TicketActionValidation TicketAction = iota
	TicketActionCollectContext
	TicketActionTransferFile
	TicketActionSendAiRequest
	TicketActionPullAiResponse
	TicketActionSendTgResponse
)

const (
	TicketTypeMessaging TicketType = iota
	TicketTypeHashingChat
)

type SuperGroupConfigModel struct {
	ChatId  int64
	OwnerId int64

	SuperGroupIds []int
}
