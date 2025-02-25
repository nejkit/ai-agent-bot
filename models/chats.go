package models

type ChatModel struct {
	ChatId        int64
	CreatorUserId int64

	ExternalGroupChatId     string           // group chats
	ExternalPersonalChatIds map[int64]string // individual chats

	ChatHash string // last formed hash by creator user id? or any participant
}

type ChatHistoryModel struct {
	ChatId  int64
	Hash    string
	Created int64
}

type ExternalChatTicketData struct {
	Id              string
	ChatId          int64
	CreatedByUserId int64

	Status TicketStatus
	Action TicketAction
	Type   TicketType

	RequestMessage  string // message from telegram chat to ai
	ReplyId         int64  // message id on chat or report recipient
	ResponseMessage string // message from ai chat to telegram chat

	ErrorMessage string // if status failed

	RetryCount int   // retry count if status failed inc
	Updated    int64 // when ticket status/action updated, unix now
	Expired    int64 //when ticket status/action updated, change by settings
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
	TicketActionSendAiRequest
	TicketActionSendTgResponse
)

const (
	TicketTypeMessaging TicketType = iota
	TicketTypeHashingChat
)
