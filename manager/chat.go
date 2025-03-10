package manager

import (
	"context"
	"errors"
	"github.com/nejkit/ai-agent-bot/models"
)

type messagesProvider interface {
	SetChatNonce(chatId int64, nonce int64) error
	GetChatNonce(chatId int64) (int64, error)
	GetMessagesForChatId(chatId int64) ([]models.MessageData, error)
	SaveMessagesForChatId(chatId int64, messages []models.MessageData) error
}

type ticketProvider interface {
	GetTicketById(ticketId string) (*models.ExternalChatTicketData, error)
	SaveTicket(data *models.ExternalChatTicketData) error
	DeleteTicket(ticketId string) error
	PushTicketIdToProcess(ticketId string) error
}

type openAICli interface {
	SendMessagesToAI(ctx context.Context, messages []models.MessageData) (string, error)
}

type ChatManager struct {
	aiCli           openAICli
	messagesStorage messagesProvider
	ticketStorage   ticketProvider
}

func NewChatManager(aiCli openAICli, messagesStorage messagesProvider, ticketStorage ticketProvider) *ChatManager {
	return &ChatManager{aiCli: aiCli, messagesStorage: messagesStorage, ticketStorage: ticketStorage}
}

func (c *ChatManager) ProcessValidationAction(ticketId string) error {
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

		if err = c.ticketStorage.PushTicketIdToProcess(ticketId); err != nil {
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

	return c.ticketStorage.PushTicketIdToProcess(ticketId)
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

	if err != nil {
		return err
	}

	chatCtx = append(chatCtx, models.MessageData{
		Text:      ticketInfo.RequestMessage,
		CreatedBy: models.MessageTypeUser,
	})

	ticketInfo.Action = models.TicketActionSendAiRequest
	ticketInfo.Status = models.TicketStatusNew
	ticketInfo.UpdateTicketExpiration()

	if err = c.ticketStorage.SaveTicket(ticketInfo); err != nil {
		return err
	}

	return c.ticketStorage.PushTicketIdToProcess(ticketId)
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

		if err = c.ticketStorage.PushTicketIdToProcess(ticketId); err != nil {
			return
		}
	}()

	return nil
}

func (c *ChatManager) ProcessActionSendToTg(ticketId string) error {
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

	ticketInfo.Status = models.TicketStatusInProgress
	ticketInfo.UpdateTicketExpiration()

	if err = c.ticketStorage.SaveTicket(ticketInfo); err != nil {
		return err
	}

	err = c.messagesStorage.SaveMessagesForChatId(ticketInfo.ChatId, ticketInfo.ChatContext)

	if err != nil {
		return err
	}

	err = c.messagesStorage.SaveMessagesForChatId(ticketInfo.ChatId, ticketInfo.ChatContext)

	if err != nil {
		return err
	}

	return nil
}
