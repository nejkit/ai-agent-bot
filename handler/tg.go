package handler

import (
	"context"
	"fmt"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"github.com/nejkit/ai-agent-bot/config"
	"github.com/nejkit/ai-agent-bot/manager"
	"github.com/nejkit/ai-agent-bot/models"
	"slices"
)

type telegramClient interface {
	SendReplyMessageForChatId(chatId int64, messageToReply int, text string) (int, error)
	EditReplyMessageForChatId(chatId int64, messageId int, text string) error
	DownloadFileById(fileId string) ([]byte, error)
}

type messagesProvider interface {
	GetMessagesForChatId(chatId int64) ([]models.MessageData, error)
	SaveMessagesForChatId(chatId int64, messages []models.MessageData) error
}

type ticketProvider interface {
	GetTicketById(ticketId string) (*models.ExternalChatTicketData, error)
	SaveTicket(data *models.ExternalChatTicketData) error
	DeleteTicket(ticketId string) error
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

			chatInfo := upd.FromChat()

			if !slices.Contains(t.cfg.AllowedChatIds, chatInfo.ID) {
				continue
			}

			go t.checkChatManagerHandler(ctx, chatInfo.ID)

			replyId, err := t.tgCLi.SendReplyMessageForChatId(chatInfo.ID, upd.Message.MessageID, "Your request queued...")

			if err != nil {
				continue
			}

			ticketModel := models.BuildTicketWithText(chatInfo.ID, replyId, upd.Message)

			if err := t.ticketProvider.SaveTicket(ticketModel); err != nil {
				continue
			}

			if err := t.ticketProvider.StoreTicketIntoPool(chatInfo.ID, ticketModel.Id, ticketModel.Request.MessageId); err != nil {
				continue
			}
		}
	}
}

func (t *TelegramHandler) checkChatManagerHandler(ctx context.Context, chatId int64) {
	chatManager, exist := t.chatManagers[chatId]
	if !exist {
		chatManager = *manager.NewChatManager(t.aiCli, t.messagesProvider, t.ticketProvider, t.tgCLi, chatId)
		t.chatManagers[chatId] = chatManager
		go chatManager.StartConsumeTickets(ctx)
	}
}

func logErrorIfFailed(err error) {
	if err != nil {
		_ = fmt.Errorf(err.Error())
	}
}
