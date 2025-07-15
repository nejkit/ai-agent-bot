package handler

import (
	"context"
	"crypto"
	"encoding/hex"
	"errors"
	"fmt"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/nejkit/ai-agent-bot/storage"
	"github.com/sirupsen/logrus"
	"slices"
	"strconv"

	"github.com/nejkit/ai-agent-bot/config"
	"github.com/nejkit/ai-agent-bot/models"
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
	actionsProvider  actionsProvider
	tgCLi            telegramClient

	chatContainer chatContainer

	cfg config.TelegramConfig
}

func NewTelegramHandler(updates tgbotapi.UpdatesChannel, ticketProvider ticketProvider, messagesProvider messagesProvider, tgCLi telegramClient, cfg config.TelegramConfig, containerManager chatContainer, actions actionsProvider) *TelegramHandler {
	return &TelegramHandler{
		updates:          updates,
		ticketProvider:   ticketProvider,
		messagesProvider: messagesProvider,
		tgCLi:            tgCLi,
		cfg:              cfg,
		chatContainer:    containerManager,
		actionsProvider:  actions,
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

	if !upd.FromChat().IsPrivate() {
		go t.tgCLi.SendReplyMessageForChatId(upd.FromChat().ID, upd.Message.MessageID, "Используйте бота в личной переписке")
		return
	}

	if upd.Message.IsCommand() {
		t.ProcessCmdUpdate(upd)
		return
	}

	action, err := t.actionsProvider.LoadAction(upd.FromChat().ID)

	if err != nil {
		return
	}

	if action != storage.ActionTypeNone {
		t.ProcessActionUpdate(action, upd)
		return
	}

	allowedChats := t.messagesProvider.GetAllowedChats()

	if !slices.Contains(allowedChats, upd.Message.Chat.ID) {
		return
	}

	chatInfo := upd.FromChat()

	if err := t.chatContainer.AddChatManager(ctx, chatInfo.ID); err != nil {
		t.tgCLi.SendReplyMessageForChatId(chatInfo.ID, upd.Message.MessageID, err.Error())
		return
	}

	ticketModel := models.BuildTicketWithText(chatInfo.ID, strconv.FormatInt(chatInfo.ID, 10), upd.Message)

	replyId, err := t.tgCLi.SendReplyMessageForChatId(chatInfo.ID, upd.Message.MessageID, "Очікуйте на виконання запиту")

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

func (t *TelegramHandler) ProcessActionUpdate(action storage.Action, upt tgbotapi.Update) {
	if action == storage.ActionTypeCheckFile && upt.Message.Document != nil {
		fileId := upt.Message.Document.FileID

		file, err := t.tgCLi.DownloadFileById(fileId)

		if err != nil {
			return
		}

		hasher := crypto.SHA3_256.New()
		hasher.Write(file)
		hash := hasher.Sum(nil)

		meta, err := t.messagesProvider.GetReportMetadata(hex.EncodeToString(hash))

		if err != nil {
			t.tgCLi.SendReplyMessageForChatId(upt.FromChat().ID, upt.Message.MessageID, fmt.Sprintf(
				"\"Хєш файла %s не знайдено в сховищі\"",
				hex.EncodeToString(hash)))
			return
		}

		t.tgCLi.SendReplyMessageForChatId(upt.FromChat().ID, upt.Message.MessageID, fmt.Sprintf(
			"Хєш знайдено, дата регістрації: %s, username автора: %s, хєш: %s",
			meta.FormDate,
			meta.Author,
			hex.EncodeToString(hash)))
		t.actionsProvider.SaveAction(upt.FromChat().ID, storage.ActionTypeNone)
	}
}

func (t *TelegramHandler) ProcessCmdUpdate(upd tgbotapi.Update) {
	if upd.Message.Command() == "cancel" {
		err := t.actionsProvider.SaveAction(upd.FromChat().ID, storage.ActionTypeNone)

		if err != nil {
			logrus.Errorln(err.Error())
		}

		return
	}

	if upd.Message.Command() == "verify_file" {
		err := t.actionsProvider.SaveAction(upd.FromChat().ID, storage.ActionTypeCheckFile)

		if err != nil {
			logrus.Errorln(err.Error())
		}

		t.tgCLi.SendReplyMessageForChatId(upd.FromChat().ID, upd.Message.MessageID, "Надішліть файл для перевірки наявності у системі")

		return
	}

	if upd.Message.Command() == "form_report" {
		allowedChats := t.messagesProvider.GetAllowedChats()

		if !slices.Contains(allowedChats, upd.Message.Chat.ID) {
			return
		}

		file, hash, err := t.chatContainer.HandleFormReport(upd.FromChat().ID)

		if err != nil {
			logrus.Errorln(err.Error())
			return
		}

		if err = t.tgCLi.SendMessageWithFile(upd.FromChat().ID, "Звіт.pdf", file); err != nil {
			logrus.Errorln(err.Error())
		}

		if err = t.tgCLi.SendMessage(upd.FromChat().ID, fmt.Sprintf(
			"Хєш документа: %s \nХєш функція: SHA3-256",
			hash,
		)); err != nil {
			logrus.Errorln(err.Error())
		}

		return
	}
}
