package handler

import (
	"context"
	"errors"
	"fmt"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/sirupsen/logrus"
	"strconv"

	"github.com/nejkit/ai-agent-bot/config"
	"github.com/nejkit/ai-agent-bot/models"
	"slices"
)

var (
	errorSupergroupCreated         = errors.New("supergroup created")
	errorCommandAllowedOnlyPrivate = errors.New("command allowed only private chats")
	errorNotAllowedUser            = errors.New("not allowed user")
)

type TelegramHandler struct {
	updates tgbotapi.UpdatesChannel

	ticketProvider   ticketProvider
	messagesProvider messagesProvider
	tgCLi            telegramClient

	chatContainer chatContainer

	cfg config.TelegramConfig
}

func NewTelegramHandler(updates tgbotapi.UpdatesChannel, ticketProvider ticketProvider, messagesProvider messagesProvider, tgCLi telegramClient, cfg config.TelegramConfig, containerManager chatContainer) *TelegramHandler {
	return &TelegramHandler{
		updates:          updates,
		ticketProvider:   ticketProvider,
		messagesProvider: messagesProvider,
		tgCLi:            tgCLi,
		cfg:              cfg,
		chatContainer:    containerManager,
	}
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

			t.processUpdate(ctx, upd)
		}
	}
}

func (t *TelegramHandler) processUpdate(ctx context.Context, upd tgbotapi.Update) {
	if upd.Message == nil {
		return
	}

	if upd.Message.IsCommand() {
		if err := t.processCommandUpdate(upd); err != nil {
			_, _ = t.tgCLi.SendReplyMessageForChatId(upd.FromChat().ID, upd.Message.MessageID, err.Error())
		}

		return
	}

	chatInfo := upd.FromChat()

	chatIds := t.messagesProvider.GetAllowedChats()

	if !slices.Contains(chatIds, chatInfo.ID) {
		return
	}

	if err := t.chatContainer.AddChatManager(ctx, chatInfo.ID); err != nil {
		t.tgCLi.SendReplyMessageForChatId(chatInfo.ID, upd.Message.MessageID, err.Error())
		return
	}

	var ticketModel *models.ExternalChatTicketData
	var err error

	if upd.FromChat().IsSuperGroup() {
		ticketModel, err = t.handleMessageFromSuperGroup(upd)

		if errors.Is(err, errorSupergroupCreated) {
			_, _ = t.tgCLi.SendReplyMessageForChatId(chatInfo.ID, upd.Message.MessageID, "Топик успешно добавлен в настройки чата")
			return
		}
	} else {
		ticketModel = models.BuildTicketWithText(chatInfo.ID, strconv.FormatInt(chatInfo.ID, 10), upd.Message)
	}

	replyId, err := t.tgCLi.SendReplyMessageForChatId(chatInfo.ID, upd.Message.MessageID, "Your request queued...")

	if err != nil {
		return
	}

	ticketModel.Response.MessageId = replyId

	if err := t.ticketProvider.SaveTicket(ticketModel); err != nil {
		return
	}

	if err := t.ticketProvider.StoreTicketIntoPool(chatInfo.ID, ticketModel.Id, ticketModel.Request.MessageId); err != nil {
		return
	}
}

func (t *TelegramHandler) handleAddChatToAllowed(chatConfig tgbotapi.ChatConfig) {
	chatInfo, err := t.tgCLi.GetChatInfoByID(chatConfig)

	if err != nil {
		logrus.Errorf("get chat info by id err: %v", err)
		return
	}

	if chatInfo.IsSuperGroup() {
		chatOwner, err := t.tgCLi.GetChatOwnerId(chatInfo.ID)

		if err != nil {
			logrus.Errorf("get chat owner id err: %v", err)
			return
		}

		err = t.messagesProvider.SaveSettingsForSuperGroupChat(chatInfo.ID, &models.SuperGroupConfigModel{
			ChatId:        chatInfo.ID,
			OwnerId:       chatOwner,
			SuperGroupIds: make([]int, 0),
		})

		if err != nil {
			logrus.Errorf("failed save settings in redis: %v", err)
			return
		}
	}

	t.messagesProvider.SaveChatToAllowed(chatInfo.ID)
}

func (t *TelegramHandler) handleGetAllowedChats(upd tgbotapi.Update) {
	chats := t.messagesProvider.GetAllowedChats()

	if len(chats) == 0 {
		t.tgCLi.SendReplyMessageForChatId(upd.FromChat().ID, upd.Message.MessageID, "Empty allowed chats")
		return
	}

	resp := ""

	for i := range chats {
		chatInfo, err := t.tgCLi.GetChatInfoByID(tgbotapi.ChatConfig{ChatID: chats[i]})

		if err != nil {
			continue
		}

		userInfo, err := t.tgCLi.GetChatOwnerInfo(chats[i])

		if err != nil {
			continue
		}

		resp += fmt.Sprintf("\n Идентификатор чата: %d Владелец: %s Название чата: %s", chatInfo.ID, userInfo.String(), chatInfo.Title)
	}

	t.tgCLi.SendReplyMessageForChatId(upd.FromChat().ID, upd.Message.MessageID, resp)
}

func (t *TelegramHandler) handleMessageFromSuperGroup(upd tgbotapi.Update) (*models.ExternalChatTicketData, error) {
	superGroupInfo, err := t.messagesProvider.GetSettingsForSuperGroupChat(upd.FromChat().ID)

	if err != nil {
		return nil, err
	}

	if upd.Message.Text == "" && upd.SentFrom().ID == superGroupInfo.OwnerId {
		superGroupInfo.SuperGroupIds = append(superGroupInfo.SuperGroupIds, upd.Message.MessageID)

		err = t.messagesProvider.SaveSettingsForSuperGroupChat(superGroupInfo.ChatId, superGroupInfo)

		if err != nil {
			return nil, err
		}

		return nil, errorSupergroupCreated
	}

	if upd.Message.ReplyToMessage == nil {
		return nil, errors.New("reply to message is empty")
	}

	if !slices.Contains(superGroupInfo.SuperGroupIds, upd.Message.ReplyToMessage.MessageID) {
		return nil, errors.New("is not topic")
	}

	ticketModel := models.BuildTicketWithText(
		superGroupInfo.ChatId,
		fmt.Sprintf("%s:%s", strconv.FormatInt(superGroupInfo.ChatId, 10), strconv.FormatInt(int64(upd.Message.ReplyToMessage.MessageID), 10)),
		upd.Message,
	)

	return ticketModel, nil
}

func (t *TelegramHandler) processCommandUpdate(upd tgbotapi.Update) error {
	if !upd.FromChat().IsPrivate() {
		return errorCommandAllowedOnlyPrivate
	}

	if !slices.Contains(t.cfg.AllowedUsers, upd.SentFrom().ID) {
		return errorNotAllowedUser
	}

	switch upd.Message.Command() {
	case "add_chat":
		cmdArgs := upd.Message.CommandArguments()

		parsedChatId, err := strconv.ParseInt(cmdArgs, 10, 64)

		var cfg tgbotapi.ChatConfig

		if err != nil {
			cfg.SuperGroupUsername = cmdArgs
		} else {
			cfg.ChatID = parsedChatId
		}

		go t.handleAddChatToAllowed(cfg)
	case "get_chats":
		go t.handleGetAllowedChats(upd)
	}

	return nil
}
