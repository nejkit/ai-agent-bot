package main

import (
	"context"
	"fmt"
	"github.com/nejkit/ai-agent-bot/manager"
	"github.com/sirupsen/logrus"
	"log"
	"time"

	"github.com/go-redis/redis"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/nejkit/ai-agent-bot/config"
	"github.com/nejkit/ai-agent-bot/handler"
	"github.com/nejkit/ai-agent-bot/provider"
	"github.com/nejkit/ai-agent-bot/storage"
	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
)

func main() {
	logrus.SetFormatter(&logrus.TextFormatter{
		FullTimestamp:   true,
		TimestampFormat: time.DateTime,
	})

	appCfg := config.GetConfig()

	redisClient := redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%d", appCfg.RedisConfig.Addr, appCfg.RedisConfig.Port),
		Password: appCfg.RedisConfig.Password,
		DB:       appCfg.RedisConfig.DB,
	})

	botApi, err := tgbotapi.NewBotAPI(appCfg.TelegramConfig.Token)

	if err != nil {
		log.Panic(err)
		return
	}

	ticketStorage := storage.NewTicketProvider(redisClient)
	messageStorage := storage.NewMessageProvider(redisClient)

	tgCli := provider.NewTelegramClient(botApi)

	aiCli := openai.NewClient(option.WithAPIKey(appCfg.AiConfig.Token))

	response, err := aiCli.Beta.Assistants.New(
		context.TODO(),
		openai.BetaAssistantNewParams{
			Model: openai.F(openai.ChatModelGPT4oMini),
			Name:  openai.F("Test"),
		},
	)

	if err != nil {
		panic(err)
	}

	openAiCli := provider.NewOpenAIClient(aiCli)
	rootCtx := context.Background()

	chatContainer := manager.NewChatManagerContainer(
		response.ID,
		ticketStorage,
		messageStorage,
		tgCli,
		openAiCli,
	)
	chatContainer.Init(rootCtx)

	updChan := botApi.GetUpdatesChan(tgbotapi.NewUpdate(0))

	handle := handler.NewTelegramHandler(updChan, ticketStorage, messageStorage, tgCli, appCfg.TelegramConfig, chatContainer)
	handle.StartHandleTgUpdates(rootCtx)
}
