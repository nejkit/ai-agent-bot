package handler

import (
	"context"
	"fmt"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
	"github.com/google/uuid"
	"github.com/nejkit/ai-agent-bot/manager"
	"github.com/nejkit/ai-agent-bot/models"
	"time"
)

type telegramClient interface {
	SendReplyMessageForChatId(chatId int64, messageToReply int, text string) (int, error)
	EditReplyMessageForChatId(chatId int64, messageId int, text string) error
}

type messagesProvider interface {
	SetChatNonce(chatId int64, nonce int) error
	GetChatNonce(chatId int64) (int, error)
	GetMessagesForChatId(chatId int64) ([]models.MessageData, error)
	SaveMessagesForChatId(chatId int64, messages []models.MessageData) error
}

type ticketProvider interface {
	GetTicketIdForProcess() (string, error)
	GetTicketById(ticketId string) (*models.ExternalChatTicketData, error)
	SaveTicket(data *models.ExternalChatTicketData) error
	DeleteTicket(ticketId string) error
	PushTicketIdToProcess(ticketId string) error
}

type openAICli interface {
	SendMessagesToAI(ctx context.Context, messages []models.MessageData) (string, error)
}

type TelegramHandler struct {
	updates      tgbotapi.UpdatesChannel
	chatManagers map[int64]manager.ChatManager

	aiCli            openAICli
	ticketProvider   ticketProvider
	messagesProvider messagesProvider
	tgCLi            telegramClient
}

func NewTelegramHandler(updates tgbotapi.UpdatesChannel, chatManagers map[int64]manager.ChatManager, aiCli openAICli, ticketProvider ticketProvider, messagesProvider messagesProvider, tgCLi telegramClient) *TelegramHandler {
	return &TelegramHandler{updates: updates, chatManagers: chatManagers, aiCli: aiCli, ticketProvider: ticketProvider, messagesProvider: messagesProvider, tgCLi: tgCLi}
}

func (t *TelegramHandler) StartHandleTgUpdates(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case upd, ok := <-t.updates:
			if !ok {
				return
			}

			fmt.Printf("Update: %v \n", upd)

			if upd.Message == nil {
				continue
			}

			if upd.Message.From.ID != 1 {
				continue
			}

			chatId := upd.Message.Chat.ID

			chatManager, exist := t.chatManagers[chatId]

			if !exist {
				chatManager = *manager.NewChatManager(t.aiCli, t.messagesProvider, t.ticketProvider, t.tgCLi)
				t.chatManagers[chatId] = chatManager

				_ = t.messagesProvider.SetChatNonce(chatId, upd.Message.MessageID)
			}

			replyMsgId, err := t.tgCLi.SendReplyMessageForChatId(chatId, upd.Message.MessageID, "Your request queued...")

			if err != nil {
				continue
			}

			ticketModel := &models.ExternalChatTicketData{
				Id:               uuid.NewString(),
				ChatId:           chatId,
				ReplyMessageId:   replyMsgId,
				Status:           models.TicketStatusNew,
				Action:           models.TicketActionValidation,
				Type:             models.TicketTypeMessaging,
				ChatContext:      make([]models.MessageData, 0),
				RequestMessage:   upd.Message.Text,
				RequestMessageId: upd.Message.MessageID,
				ResponseMessage:  "",
				ErrorMessage:     "",
				RetryCount:       0,
				Updated:          time.Now().UnixMilli(),
				Expired:          time.Now().Add(time.Hour).UnixMilli(),
			}

			if err := t.ticketProvider.SaveTicket(ticketModel); err != nil {
				continue
			}

			if err := t.ticketProvider.PushTicketIdToProcess(ticketModel.Id); err != nil {
				continue
			}
		}
	}
}

func (t *TelegramHandler) StartProcessTickets(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
			ticketId, err := t.ticketProvider.GetTicketIdForProcess()

			if err != nil {
				continue
			}

			ticketInfo, err := t.ticketProvider.GetTicketById(ticketId)

			if err != nil {
				continue
			}

			chatManager, exist := t.chatManagers[ticketInfo.ChatId]

			if !exist {
				continue
			}

			switch ticketInfo.Action {
			case models.TicketActionValidation:
				go logErrorIfFailed(chatManager.ProcessValidationAction(ticketId))
			case models.TicketActionCollectContext:
				go logErrorIfFailed(chatManager.ProcessCollectContextAction(ticketId))
			case models.TicketActionSendAiRequest:
				go logErrorIfFailed(chatManager.ProcessActionSendToAi(ticketId))
			case models.TicketActionSendTgResponse:
				go logErrorIfFailed(chatManager.ProcessActionSendToTg(ticketId))
			}
		}
	}
}

func logErrorIfFailed(err error) {
	if err != nil {
		_ = fmt.Errorf(err.Error())
	}
}
