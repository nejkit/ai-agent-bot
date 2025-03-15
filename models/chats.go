package models

import (
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/google/uuid"
	"time"
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

func BuildTicketWithText(chatId int64, replyId int, message *tgbotapi.Message) *ExternalChatTicketData {
	fileId := ""

	if message.Document != nil {
		fileId = message.Document.FileID
	}

	if message.Photo != nil {
		fileId = message.Photo[0].FileID
	}

	return &ExternalChatTicketData{
		Id:          uuid.NewString(),
		ChatId:      chatId,
		Status:      TicketStatusNew,
		Action:      TicketActionValidation,
		Type:        TicketTypeMessaging,
		ChatContext: make([]MessageData, 0),
		Request: RequestData{
			Text:      message.Text,
			FileId:    fileId,
			MessageId: message.MessageID,
		},
		Response: ResponseData{
			MessageId: replyId,
		},
		RetryCount: 0,
		Updated:    time.Now().UnixMilli(),
		Expired:    time.Now().Add(time.Hour).UnixMilli(),
	}
}

type ExternalChatTicketData struct {
	Id     string
	ChatId int64

	Status TicketStatus
	Action TicketAction
	Type   TicketType

	ChatContext []MessageData

	Request  RequestData
	Response ResponseData

	Error error

	RetryCount int   // retry count if status failed inc
	Updated    int64 // when ticket status/action updated, unix now

	Expired int64 //when ticket status/action updated, change by settings
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

func (t *ExternalChatTicketData) UpdateTicketExpiration() {
	t.Expired = time.Now().Add(time.Hour).UnixMilli()
}

type TicketStatus int
type TicketAction int
type TicketType int

const (
	TicketStatusNew TicketStatus = iota
	TicketStatusInProgress
	TicketStatusWaitResponse
	TicketStatusDone
	TicketStatusError
)

const (
	TicketActionValidation TicketAction = iota
	TicketActionCollectContext
	TicketActionSendAiRequest
	TicketActionSendTgResponse
)

const (
	TicketTypeMessaging TicketType = iota
	TicketTypeHashingChat
)
