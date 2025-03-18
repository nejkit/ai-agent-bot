package manager

import (
	"context"
	"sync"
)

type ChatManagerContainer struct {
	activeRoutines map[int64]context.CancelFunc

	aiCli           openAICli
	tgCli           telegramClient
	messagesStorage messagesProvider
	ticketStorage   ticketProvider

	assistantId string
	mtx         *sync.Mutex
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
