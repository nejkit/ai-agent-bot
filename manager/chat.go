package manager

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/nejkit/ai-agent-bot/models"
	"github.com/nejkit/ai-agent-bot/storage"
)

type telegramClient interface {
	EditReplyMessageForChatId(chatId int64, messageId int, text string) error
}

type messagesProvider interface {
	SetChatNonce(chatId int64, nonce int) error
	GetChatNonce(chatId int64) (int, error)
	GetMessagesForChatId(chatId int64) ([]models.MessageData, error)
	SaveMessagesForChatId(chatId int64, messages []models.MessageData) error
}

type ticketProvider interface {
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

type ChatManager struct {
	aiCli           openAICli
	tgCli           telegramClient
	messagesStorage messagesProvider
	ticketStorage   ticketProvider
	mtx             *sync.RWMutex
	chatId          int64
}

func NewChatManager(aiCli openAICli, messagesStorage messagesProvider, ticketStorage ticketProvider, tgCli telegramClient, chatId int64) *ChatManager {
	return &ChatManager{aiCli: aiCli, messagesStorage: messagesStorage, ticketStorage: ticketStorage, tgCli: tgCli, mtx: &sync.RWMutex{}, chatId: chatId}
}

func (c *ChatManager) StartConsumeTickets(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
			ticketId, err := c.ticketStorage.GetTicketFromPool(c.chatId)

			if err != nil {
				time.Sleep(time.Millisecond * 30)
				continue
			}

			ticketInfo, err := c.ticketStorage.GetTicketById(ticketId)

			if err != nil {
				time.Sleep(time.Millisecond * 30)
				continue
			}

			switch ticketInfo.Action {
			case models.TicketActionValidation:
				c.ProcessValidationAction(ticketId)
			case models.TicketActionCollectContext:
				c.ProcessCollectContextAction(ticketId)
			case models.TicketActionSendAiRequest:
				c.ProcessActionSendToAi(ticketId)
			case models.TicketActionSendTgResponse:
				c.ProcessActionSendToTg(ticketId)
			}

			time.Sleep(time.Millisecond * 30)
		}
	}
}

func (c *ChatManager) ProcessValidationAction(ticketId string) error {
	c.mtx.Lock()
	defer c.mtx.Unlock()
	ticketInfo, err := c.ticketStorage.GetTicketById(ticketId)

	if err != nil {
		return err
	}

	if ticketInfo.Status != models.TicketStatusNew {
		return errors.New("ticket status is not new")
	}

	if ticketInfo.Action != models.TicketActionValidation {
		return errors.New("ticket action is not valid")
	}

	ticketInfo.Status = models.TicketStatusInProgress
	ticketInfo.UpdateTicketExpiration()

	if err = c.ticketStorage.SaveTicket(ticketInfo); err != nil {
		return err
	}

	chatNonce, err := c.messagesStorage.GetChatNonce(ticketInfo.ChatId)
	if err != nil {
		return err
	}

	if chatNonce < ticketInfo.RequestMessageId {
		ticketInfo.Status = models.TicketStatusNew

		if err = c.ticketStorage.SaveTicket(ticketInfo); err != nil {
			return err
		}

		if err = c.ticketStorage.StoreTicketIntoPool(c.chatId, ticketId, ticketInfo.RequestMessageId); err != nil {
			return err
		}

		return errors.New("chat nonce is too small")
	}

	ticketInfo.Action = models.TicketActionCollectContext
	ticketInfo.Status = models.TicketStatusNew
	ticketInfo.UpdateTicketExpiration()

	if err = c.ticketStorage.SaveTicket(ticketInfo); err != nil {
		return err
	}

	return c.ticketStorage.StoreTicketIntoPool(c.chatId, ticketId, ticketInfo.RequestMessageId)
}

func (c *ChatManager) ProcessCollectContextAction(ticketId string) error {
	ticketInfo, err := c.ticketStorage.GetTicketById(ticketId)

	if err != nil {
		return err
	}

	if ticketInfo.Status != models.TicketStatusNew {
		return errors.New("ticket status is not new")
	}

	if ticketInfo.Action != models.TicketActionCollectContext {
		return errors.New("ticket action is not valid")
	}

	ticketInfo.Status = models.TicketStatusInProgress
	ticketInfo.UpdateTicketExpiration()

	if err = c.ticketStorage.SaveTicket(ticketInfo); err != nil {
		return err
	}

	chatCtx, err := c.messagesStorage.GetMessagesForChatId(ticketInfo.ChatId)

	if err != nil && !errors.Is(err, storage.ErrorNotFound) {
		return err
	}

	if errors.Is(err, storage.ErrorNotFound) {
		chatCtx = make([]models.MessageData, 0)
	}

	chatCtx = append(chatCtx, models.MessageData{
		Text:      ticketInfo.RequestMessage,
		CreatedBy: models.MessageTypeUser,
	})

	ticketInfo.ChatContext = chatCtx

	if len(chatCtx) > 10 {
		ticketInfo.ChatContext = chatCtx[len(chatCtx)-10:]
	}

	ticketInfo.Action = models.TicketActionSendAiRequest
	ticketInfo.Status = models.TicketStatusNew

	ticketInfo.UpdateTicketExpiration()

	if err = c.ticketStorage.SaveTicket(ticketInfo); err != nil {
		return err
	}

	return c.ticketStorage.StoreTicketIntoPool(c.chatId, ticketId, ticketInfo.RequestMessageId)
}

func (c *ChatManager) ProcessActionSendToAi(ticketId string) error {
	ticketInfo, err := c.ticketStorage.GetTicketById(ticketId)

	if err != nil {
		return err
	}

	if ticketInfo.Status != models.TicketStatusNew {
		return errors.New("ticket status is not new")
	}

	if ticketInfo.Action != models.TicketActionSendAiRequest {
		return errors.New("ticket action is not valid")
	}

	ticketInfo.Status = models.TicketStatusWaitResponse
	ticketInfo.UpdateTicketExpiration()

	if err = c.ticketStorage.SaveTicket(ticketInfo); err != nil {
		return err
	}

	go func() {
		response, err := c.aiCli.SendMessagesToAI(context.TODO(), ticketInfo.ChatContext)

		if err != nil {

			if ticketInfo.RetryCount >= 3 {
				ticketInfo.Status = models.TicketStatusError
				ticketInfo.ErrorMessage = err.Error()
				return
			}

			ticketInfo.Status = models.TicketStatusNew
			ticketInfo.ErrorMessage = err.Error()
			ticketInfo.RetryCount++
			ticketInfo.UpdateTicketExpiration()

			if err := c.ticketStorage.SaveTicket(ticketInfo); err != nil {
				return
			}
		}

		ticketInfo.Status = models.TicketStatusNew
		ticketInfo.UpdateTicketExpiration()
		ticketInfo.Action = models.TicketActionSendTgResponse
		ticketInfo.ResponseMessage = response
		ticketInfo.ChatContext = append(ticketInfo.ChatContext, models.MessageData{
			Text:      response,
			CreatedBy: models.MessageTypeAssistant,
		})

		if err = c.ticketStorage.SaveTicket(ticketInfo); err != nil {
			return
		}

		if err = c.ticketStorage.StoreTicketIntoPool(c.chatId, ticketId, ticketInfo.RequestMessageId); err != nil {
			return
		}
	}()

	return nil
}

func (c *ChatManager) ProcessActionSendToTg(ticketId string) error {
	c.mtx.Lock()
	defer c.mtx.Unlock()
	ticketInfo, err := c.ticketStorage.GetTicketById(ticketId)

	if err != nil {
		return err
	}

	if ticketInfo.Status != models.TicketStatusNew {
		return errors.New("ticket status is not new")
	}

	if ticketInfo.Action != models.TicketActionSendTgResponse {
		return errors.New("ticket action is not valid")
	}

	ticketInfo.Status = models.TicketStatusInProgress
	ticketInfo.UpdateTicketExpiration()

	if err = c.ticketStorage.SaveTicket(ticketInfo); err != nil {
		return err
	}

	err = c.messagesStorage.SaveMessagesForChatId(ticketInfo.ChatId, ticketInfo.ChatContext)

	if err != nil {
		return err
	}

	nonce, err := c.ticketStorage.GetMinimumScoreFromPool(c.chatId)

	if err != nil && !errors.Is(err, storage.ErrorNotFound) {
		return err
	}

	if errors.Is(err, storage.ErrorNotFound) {
		nonce = ticketInfo.ReplyMessageId + 1
	}

	err = c.messagesStorage.SetChatNonce(ticketInfo.ChatId, nonce)

	if err != nil {
		return err
	}

	err = c.tgCli.EditReplyMessageForChatId(ticketInfo.ChatId, ticketInfo.ReplyMessageId, ticketInfo.ResponseMessage)

	return c.ticketStorage.DeleteTicket(ticketId)
}
