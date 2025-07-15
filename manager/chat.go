package manager

import (
	"context"
	"errors"
	"github.com/sirupsen/logrus"
	"os"
	"sync"
	"time"

	"github.com/nejkit/ai-agent-bot/models"
	"github.com/nejkit/ai-agent-bot/storage"
)

func getLogger(ticketID string) *logrus.Entry {
	return logrus.WithField("ticket-id", ticketID)
}

type ChatManager struct {
	aiCli           openAICli
	tgCli           telegramClient
	messagesStorage messagesProvider
	ticketStorage   ticketProvider
	mtx             *sync.RWMutex
	chatId          int64
	assistantId     string
}

func NewChatManager(aiCli openAICli, messagesStorage messagesProvider, ticketStorage ticketProvider, tgCli telegramClient, chatId int64, assistantId string) *ChatManager {
	return &ChatManager{
		aiCli:           aiCli,
		messagesStorage: messagesStorage,
		ticketStorage:   ticketStorage,
		tgCli:           tgCli,

		assistantId: assistantId,
		chatId:      chatId,

		mtx: &sync.RWMutex{},
	}
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

			if errors.Is(err, storage.ErrorNotFound) {
				_ = c.ticketStorage.DeleteTicketFromPool(c.chatId, ticketId)
				continue
			}

			if err != nil {
				time.Sleep(time.Millisecond * 30)
				continue
			}

			if (ticketInfo.Status == models.TicketStatusInProgress || ticketInfo.Status == models.TicketStatusWaitResponse) && time.UnixMilli(ticketInfo.RetryAt).Before(time.Now()) {
				ticketInfo.Status = models.TicketStatusNew
				ticketInfo.RetryCount += 1

				if ticketInfo.RetryCount > 2 {
					if ticketInfo.Action == models.TicketActionSendTgResponse {
						_ = c.ticketStorage.DeleteTicket(ticketId)
						continue
					}

					ticketInfo.Response.Text = "Timeout process ticket"
					ticketInfo.Action = models.TicketActionSendTgResponse
				}

				_ = c.ticketStorage.SaveTicket(ticketInfo)
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
			case models.TicketActionTransferFile:
				c.ProcessActionTransferFileToAi(ticketId)
			case models.TicketActionPullAiResponse:
				c.ProcessActionPollAiResponse(ticketId)
			}

			time.Sleep(time.Millisecond * 1000)
		}
	}
}

func (c *ChatManager) ProcessValidationAction(ticketId string) error {
	c.mtx.Lock()
	defer c.mtx.Unlock()

	getLogger(ticketId).Infoln("Try get ticket data for validation")

	ticketInfo, err := c.ticketStorage.GetTicketById(ticketId)

	if err != nil {
		return err
	}

	if ticketInfo.Status != models.TicketStatusNew {
		getLogger(ticketId).Warningln("Ticket already process, skip...")
		return errors.New("ticket status is not new")
	}

	if ticketInfo.Action != models.TicketActionValidation {
		getLogger(ticketId).Infoln("Ticket already validated, skip...")
		return errors.New("ticket action is not valid")
	}

	ticketInfo.Status = models.TicketStatusInProgress
	ticketInfo.RetryAt = time.Now().Add(time.Minute).UnixMilli()

	if err = c.ticketStorage.SaveTicket(ticketInfo); err != nil {
		return err
	}

	ticketInfo.Action = models.TicketActionCollectContext
	ticketInfo.Status = models.TicketStatusNew
	ticketInfo.RetryCount = 0

	getLogger(ticketId).Infof("Ticket successfully validated")

	return c.ticketStorage.SaveTicket(ticketInfo)
}

func (c *ChatManager) ProcessCollectContextAction(ticketId string) error {
	getLogger(ticketId).Infoln("Try get ticket data for collect chatCtx")
	ticketInfo, err := c.ticketStorage.GetTicketById(ticketId)

	if err != nil {
		return err
	}

	if ticketInfo.Status != models.TicketStatusNew {
		getLogger(ticketId).Warningln("Ticket already process, skip...")
		return errors.New("ticket status is not new")
	}

	if ticketInfo.Action != models.TicketActionCollectContext {
		getLogger(ticketId).Warningln("Ticket already processed, skip...")
		return errors.New("ticket action is not valid")
	}

	ticketInfo.Status = models.TicketStatusInProgress
	ticketInfo.RetryAt = time.Now().Add(time.Minute).UnixMilli()

	if err = c.ticketStorage.SaveTicket(ticketInfo); err != nil {
		return err
	}

	chatCtx, err := c.messagesStorage.GetMessagesForChatId(ticketInfo.ChatContextKey)

	if err != nil && !errors.Is(err, storage.ErrorNotFound) {
		return err
	}

	if errors.Is(err, storage.ErrorNotFound) {
		chatCtx = make([]models.MessageData, 0)
	}

	chatCtx = append(chatCtx, models.MessageData{
		Text:      ticketInfo.Request.Text,
		CreatedBy: models.MessageTypeUser,
	})

	ticketInfo.ChatContext = chatCtx

	if len(chatCtx) > 10 {
		ticketInfo.ChatContext = chatCtx[len(chatCtx)-10:]
	}

	ticketInfo.Status = models.TicketStatusNew
	ticketInfo.RetryCount = 0

	getLogger(ticketId).Infoln("Ticket successfully filled chatCtx")

	if ticketInfo.Request.FileId != "" {
		ticketInfo.Action = models.TicketActionTransferFile

		return c.ticketStorage.SaveTicket(ticketInfo)
	}

	ticketInfo.Action = models.TicketActionSendAiRequest

	return c.ticketStorage.SaveTicket(ticketInfo)
}

func (c *ChatManager) ProcessActionSendToAi(ticketId string) error {
	getLogger(ticketId).Infoln("Try get ticket data for send AI Request")
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

	if ticketInfo.AssistantData != nil {
		getLogger(ticketId).Infoln("Ticket contains file, try process with file")
		return c.processActionSendToAiWithFile(ticketInfo)
	}

	ticketInfo.Status = models.TicketStatusWaitResponse
	ticketInfo.RetryAt = time.Now().Add(time.Minute).UnixMilli()

	if err = c.ticketStorage.SaveTicket(ticketInfo); err != nil {
		return err
	}

	go func() {
		response, err := c.aiCli.SendMessagesToAI(context.TODO(), ticketInfo.ChatContext)

		if err != nil {
			getLogger(ticketId).Infoln("Ticket failed on action send ai request with error: ", err.Error())
			ticketInfo.Status = models.TicketStatusError
			ticketInfo.Response.Text = err.Error()
			ticketInfo.RetryCount = 0
			ticketInfo.Action = models.TicketActionSendTgResponse

			if err := c.ticketStorage.SaveTicket(ticketInfo); err != nil {
				return
			}
		}

		getLogger(ticketId).Infoln("Ai response success, try update ticket data")

		ticketInfo.Status = models.TicketStatusNew
		ticketInfo.Action = models.TicketActionSendTgResponse
		ticketInfo.RetryCount = 0
		ticketInfo.Response.Text = response
		ticketInfo.ChatContext = append(ticketInfo.ChatContext, models.MessageData{
			Text:      response,
			CreatedBy: models.MessageTypeAssistant,
		})

		if err = c.ticketStorage.SaveTicket(ticketInfo); err != nil {
			return
		}

		getLogger(ticketId).Infoln("Ticket data saved")
	}()

	return nil
}

func (c *ChatManager) processActionSendToAiWithFile(ticketInfo *models.ExternalChatTicketData) error {
	threadId, runId, err := c.aiCli.SendMessageWithFileToAI(
		context.Background(),
		ticketInfo.ChatContext,
		c.assistantId,
		ticketInfo.AssistantData.FileId,
	)

	if err != nil {
		getLogger(ticketInfo.Id).Errorln("Failed start assistant API, error: ", err.Error())
		return err
	}

	ticketInfo.AssistantData.RunId = runId
	ticketInfo.AssistantData.ThreadId = threadId

	ticketInfo.Status = models.TicketStatusWaitResponse
	ticketInfo.Action = models.TicketActionPullAiResponse
	ticketInfo.RetryCount = 0

	return c.ticketStorage.SaveTicket(ticketInfo)
}

func (c *ChatManager) ProcessActionPollAiResponse(ticketId string) error {
	getLogger(ticketId).Infoln("Try get ticket data for polling ai run status")
	ticketInfo, err := c.ticketStorage.GetTicketById(ticketId)

	if err != nil {
		return err
	}

	if ticketInfo.Status != models.TicketStatusWaitResponse {
		return errors.New("ticket status is not wait response")
	}

	if ticketInfo.Action != models.TicketActionPullAiResponse {
		return errors.New("ticket action is not valid")
	}

	ticketInfo.Status = models.TicketStatusInProgress
	ticketInfo.RetryAt = time.Now().Add(time.Minute).UnixMilli()

	if err = c.ticketStorage.SaveTicket(ticketInfo); err != nil {
		getLogger(ticketInfo.Id).Errorln("Failed save ticket state, error: ", err.Error())
		return err
	}

	response, fileId, err := c.aiCli.PollResponseFromAssistant(context.TODO(), ticketInfo.AssistantData.ThreadId, ticketInfo.AssistantData.RunId)

	if err != nil {
		getLogger(ticketId).Errorln("Failed polled run statue, error: ", err.Error())
		return err
	}

	getLogger(ticketId).Infoln("Success polled ai response")

	ticketInfo.Response.Text = response

	if fileId != "" {
		getLogger(ticketId).Infoln("Ai response contains file, try download and save in ticket data")
		content, err := c.aiCli.DownloadFile(context.TODO(), fileId)

		if err != nil {
			getLogger(ticketId).Infoln("Failed downloading file, error: ", err.Error())
			return err
		}

		ticketInfo.Response.FileContent = content
	}

	ticketInfo.Status = models.TicketStatusNew
	ticketInfo.Action = models.TicketActionSendTgResponse

	return c.ticketStorage.SaveTicket(ticketInfo)
}

func (c *ChatManager) ProcessActionTransferFileToAi(ticketId string) error {
	c.mtx.Lock()
	defer c.mtx.Unlock()

	getLogger(ticketId).Infoln("Try get ticket data for transfer file from telegram to AI")

	ticketInfo, err := c.ticketStorage.GetTicketById(ticketId)

	if err != nil {
		return err
	}

	if ticketInfo.Status != models.TicketStatusNew {
		return errors.New("ticket status is not new")
	}

	if ticketInfo.Action != models.TicketActionTransferFile {
		return errors.New("ticket action is not valid")
	}

	ticketInfo.Status = models.TicketStatusInProgress
	ticketInfo.RetryAt = time.Now().Add(time.Minute).UnixMilli()

	if err = c.ticketStorage.SaveTicket(ticketInfo); err != nil {
		return err
	}

	fileContent, err := c.tgCli.DownloadFileById(ticketInfo.Request.FileId)

	os.WriteFile("test.pdf", fileContent, os.ModeAppend)

	if err != nil {
		ticketInfo.Status = models.TicketStatusError
		ticketInfo.Error = err

		getLogger(ticketId).Errorln("Failed downloading file from telegram, error: ", err.Error())

		if err := c.ticketStorage.SaveTicket(ticketInfo); err != nil {
			return err
		}

		return err
	}

	getLogger(ticketId).Infoln("Success download file, try upload on openAI")

	aiFileId, err := c.aiCli.UploadFile(context.Background(), fileContent)

	if err != nil {
		getLogger(ticketId).Errorln("Failed upload file to AI, error: ", err.Error())
		ticketInfo.Status = models.TicketStatusError
		ticketInfo.Error = err

		if err := c.ticketStorage.SaveTicket(ticketInfo); err != nil {
			return err
		}

		return err
	}

	getLogger(ticketId).Infoln("Success download file to openAI")

	ticketInfo.AssistantData = &models.AssistantData{
		FileId: aiFileId,
	}

	ticketInfo.Status = models.TicketStatusNew
	ticketInfo.Action = models.TicketActionSendAiRequest
	ticketInfo.RetryCount = 0

	return c.ticketStorage.SaveTicket(ticketInfo)
}

func (c *ChatManager) ProcessActionSendToTg(ticketId string) error {
	c.mtx.Lock()
	defer c.mtx.Unlock()

	getLogger(ticketId).Infoln("Get ticket data for send telegram message")

	ticketInfo, err := c.ticketStorage.GetTicketById(ticketId)

	if err != nil {
		return err
	}

	if ticketInfo.Status == models.TicketStatusError {
		err = c.tgCli.EditReplyMessageForChatId(ticketInfo.ChatId, ticketInfo.Response.MessageId, ticketInfo.Response.Text)

		if err != nil {

			getLogger(ticketId).Infoln("Failed edit response to chat")
			return err
		}

		return c.ticketStorage.DeleteTicket(ticketId)
	}

	if ticketInfo.Status != models.TicketStatusNew {
		return errors.New("ticket status is not new")
	}

	if ticketInfo.Action != models.TicketActionSendTgResponse {
		return errors.New("ticket action is not valid")
	}

	ticketInfo.Status = models.TicketStatusInProgress
	ticketInfo.RetryAt = time.Now().Add(time.Minute).UnixMilli()

	if err = c.ticketStorage.SaveTicket(ticketInfo); err != nil {
		return err
	}

	err = c.messagesStorage.SaveMessagesForChatId(ticketInfo.ChatContextKey, ticketInfo.ChatContext)

	if err != nil {
		getLogger(ticketId).Errorln("Failed save chatCtx to storage")
		return err
	}

	err = c.tgCli.EditReplyMessageForChatId(ticketInfo.ChatId, ticketInfo.Response.MessageId, ticketInfo.Response.Text)

	if err != nil {

		getLogger(ticketId).Infoln("Failed edit response to chat")
		return err
	}

	getLogger(ticketId).Infoln("Request success processed, try delete ticket data from storage")

	return c.ticketStorage.DeleteTicket(ticketId)
}
