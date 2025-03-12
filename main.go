package main

import (
	"context"
	"fmt"
	"log"

	"github.com/go-redis/redis"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/nejkit/ai-agent-bot/config"
	"github.com/nejkit/ai-agent-bot/handler"
	"github.com/nejkit/ai-agent-bot/manager"
	"github.com/nejkit/ai-agent-bot/provider"
	"github.com/nejkit/ai-agent-bot/storage"
	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
)

func main() {
	config := config.AppConfig{}

	redisClient := redis.NewClient(&redis.Options{
		Addr:     config.RedisConfig.Addr,
		Password: config.RedisConfig.Password,
		DB:       config.RedisConfig.DB,
	})

	botApi, err := tgbotapi.NewBotAPI(config.TelegramConfig.Token)

	if err != nil {
		log.Panic(err)
		return
	}

	ticketStorage := storage.NewTicketProvider(redisClient)
	messageStorage := storage.NewMessageProvider(redisClient)

	tgCli := provider.NewTelegramClient(botApi)

	aiCli := openai.NewClient(option.WithAPIKey(config.AiConfig.Token))

	openAiCli := provider.NewOpenAIClient(aiCli)

	updChan := botApi.GetUpdatesChan(tgbotapi.NewUpdate(0))

	handle := handler.NewTelegramHandler(updChan, make(map[int64]manager.ChatManager), openAiCli, ticketStorage, messageStorage, tgCli, config.TelegramConfig)

	fmt.Println("Start app")
	handle.StartHandleTgUpdates(context.Background())

}
