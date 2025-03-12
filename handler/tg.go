package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/google/uuid"
	"github.com/nejkit/ai-agent-bot/config"
	"github.com/nejkit/ai-agent-bot/manager"
	"github.com/nejkit/ai-agent-bot/models"
	"github.com/sirupsen/logrus"
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
	GetMinimumScoreFromPool(chatId int64) (int, error)
	GetTicketFromPool(chatId int64) (string, error)
	StoreTicketIntoPool(chatId int64, ticketId string, requestMessageId int) error
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
	cfg              config.TelegramConfig
}

func NewTelegramHandler(updates tgbotapi.UpdatesChannel, chatManagers map[int64]manager.ChatManager, aiCli openAICli, ticketProvider ticketProvider, messagesProvider messagesProvider, tgCLi telegramClient, cfg config.TelegramConfig) *TelegramHandler {
	return &TelegramHandler{updates: updates, chatManagers: chatManagers, aiCli: aiCli, ticketProvider: ticketProvider, messagesProvider: messagesProvider, tgCLi: tgCLi, cfg: cfg}
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
			chatId := upd.Message.Chat.ID

			if upd.Message.Chat.IsSuperGroup() {
				jsonData, _ := json.Marshal(upd.Message)
				logrus.Infof("Supergroup topic: %v", string(jsonData))
			}

			isAuthorized := false

			for _, id := range t.cfg.AllowedChatIds {
				if id == chatId {
					isAuthorized = true
					break
				}
			}

			if !isAuthorized {
				continue
			}

			chatManager, exist := t.chatManagers[chatId]
			if !exist {
				chatManager = *manager.NewChatManager(t.aiCli, t.messagesProvider, t.ticketProvider, t.tgCLi, chatId)
				t.chatManagers[chatId] = chatManager

				_ = t.messagesProvider.SetChatNonce(chatId, upd.Message.MessageID)
				go chatManager.StartConsumeTickets(ctx)
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

			if err := t.ticketProvider.StoreTicketIntoPool(chatId, ticketModel.Id, ticketModel.RequestMessageId); err != nil {
				continue
			}
		}
	}
}

func logErrorIfFailed(err error) {
	if err != nil {
		_ = fmt.Errorf(err.Error())
	}
}
