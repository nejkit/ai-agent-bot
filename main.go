package main

import (
	"context"
	"github.com/go-redis/redis"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
	config2 "github.com/nejkit/ai-agent-bot/config"
	handler2 "github.com/nejkit/ai-agent-bot/handler"
	"github.com/nejkit/ai-agent-bot/manager"
	"github.com/nejkit/ai-agent-bot/provider"
	"github.com/nejkit/ai-agent-bot/storage"
	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
	"log"
)

func main() {
	config := config2.AppConfig{
		TelegramConfig: config2.TelegramConfig{
			Token: "",
		},
		RedisConfig: config2.RedisConfig{
			Addr:     "",
			Password: "",
			DB:       0,
		},
	}

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

	updChan, err := botApi.GetUpdatesChan(tgbotapi.NewUpdate(0))

	if err != nil {
		log.Panic(err)
		return
	}

	handler := handler2.NewTelegramHandler(updChan, make(map[int64]manager.ChatManager), openAiCli, ticketStorage, messageStorage, tgCli)

	go handler.StartProcessTickets(context.Background())
	go handler.StartHandleTgUpdates(context.Background())
}
