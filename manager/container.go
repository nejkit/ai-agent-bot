package manager

import (
	"context"
	"crypto"
	"encoding/hex"
	"fmt"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/nejkit/ai-agent-bot/models"
	"github.com/nejkit/ai-agent-bot/provider"
	"github.com/robfig/cron/v3"
	"io"
	"os"
	"strconv"
	"sync"
	"time"
)

type ChatManagerContainer struct {
	activeRoutines map[int64]context.CancelFunc

	aiCli           openAICli
	tgCli           telegramClient
	messagesStorage messagesProvider
	ticketStorage   ticketProvider

	assistantId string
	mtx         *sync.Mutex
	reportMtx   *sync.Mutex
}

func NewChatManagerContainer(assistantId string, ticketStorage ticketProvider, messagesStorage messagesProvider, tgCli telegramClient, aiCli openAICli) *ChatManagerContainer {
	return &ChatManagerContainer{
		assistantId:     assistantId,
		ticketStorage:   ticketStorage,
		messagesStorage: messagesStorage,
		tgCli:           tgCli,
		aiCli:           aiCli,
		activeRoutines:  make(map[int64]context.CancelFunc),

		mtx: &sync.Mutex{},
	}
}

func (c *ChatManagerContainer) Init(rootCtx context.Context) {
	c.mtx.Lock()
	defer c.mtx.Unlock()

	chats := c.messagesStorage.GetAllowedChats()

	for _, chat := range chats {
		ctx, cancel := context.WithCancel(rootCtx)

		_, exist := c.activeRoutines[chat]

		if exist {
			cancel()
			continue
		}

		manager := NewChatManager(c.aiCli, c.messagesStorage, c.ticketStorage, c.tgCli, chat, c.assistantId)

		c.activeRoutines[chat] = cancel

		go manager.StartConsumeTickets(ctx)
	}

	cron2 := cron.New()

	_, err := cron2.AddFunc(os.Getenv("CRON_INTERVAL"), func() {
		for chat := range c.activeRoutines {
			file, hash, err := c.HandleFormReport(chat)

			if err != nil {
				getLogger("todo").Errorln(err.Error())
				continue
			}

			if err = c.tgCli.SendMessageWithFile(chat, "Звіт.pdf", file); err != nil {
				getLogger("todo").Errorln(err.Error())
			}

			if err = c.tgCli.SendMessage(chat, fmt.Sprintf(
				"Хєш документа: %s \nХєш функція: SHA3-256",
				hash,
			),
			); err != nil {
				getLogger("todo").Errorln(err.Error())
			}
		}
	})
	if err != nil {
		panic(err)
	}

	cron2.Start()
}

func (c *ChatManagerContainer) AddChatManager(rootCtx context.Context, chatID int64) error {
	c.mtx.Lock()
	defer c.mtx.Unlock()

	_, exist := c.activeRoutines[chatID]

	if exist {
		return nil
	}

	ctx, cancel := context.WithCancel(rootCtx)

	chatManager := NewChatManager(c.aiCli, c.messagesStorage, c.ticketStorage, c.tgCli, chatID, c.assistantId)

	c.activeRoutines[chatID] = cancel
	go chatManager.StartConsumeTickets(ctx)

	return nil
}

func (c *ChatManagerContainer) RemoveChatManager(chatID int64) error {
	c.mtx.Lock()
	defer c.mtx.Unlock()

	cancel, exist := c.activeRoutines[chatID]

	if !exist {
		return nil
	}

	cancel()

	delete(c.activeRoutines, chatID)

	return nil
}

type nopCloser struct {
	io.Writer
}

func (nopCloser) Close() error {
	return nil
}

func (c *ChatManagerContainer) HandleFormReport(chatId int64) ([]byte, string, error) {
	messages, err := c.messagesStorage.GetMessagesForChatId(strconv.FormatInt(chatId, 10))

	if err != nil {
		return nil, "", err
	}

	messages = append(messages, models.MessageData{
		Text:      "Треба зформувати звіт щодо нашого спілкування, проаналізуй запити і відповіді, та надай текстовий фідбєк щодо моєї активності, спрогнозуй мій подальший зріст, спираючись на активність та запити",
		CreatedBy: models.MessageTypeUser,
	})

	response, err := c.aiCli.SendMessagesToAI(context.Background(), messages)
	formDate := time.Now().UTC().Format("2006-01-02 15:04") + " UTC"

	file, err := provider.GeneratePdf(response, formDate)

	if err != nil {
		return nil, "", err
	}

	hasher := crypto.SHA3_256.New()
	hasher.Write(file)
	hash := hasher.Sum(nil)
	chatInfo, _ := c.tgCli.GetChatInfoByID(tgbotapi.ChatConfig{ChatID: chatId})

	c.messagesStorage.SaveReportMeta(hex.EncodeToString(hash), chatInfo.UserName, formDate)

	return file, hex.EncodeToString(hash), nil
}
