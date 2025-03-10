package models

import "time"

type MessageData struct {
	Text      string
	CreatedBy MessageType
}

type MessageType int

const (
	MessageTypeUser = iota
	MessageTypeAssistant
)

type ExternalChatTicketData struct {
	Id     string
	ChatId int64

	ReplyMessageId int

	Status TicketStatus
	Action TicketAction
	Type   TicketType

	ChatContext []MessageData

	RequestMessage   string
	RequestMessageId int

	ResponseMessage string // message from ai chat to telegram chat

	ErrorMessage string // if status failed

	RetryCount int   // retry count if status failed inc
	Updated    int64 // when ticket status/action updated, unix now
	Expired    int64 //when ticket status/action updated, change by settings
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
