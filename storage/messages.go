package storage

import (
	"encoding/json"
	"errors"
	"github.com/go-redis/redis"
	"github.com/nejkit/ai-agent-bot/models"
)

type MessageProvider struct {
	cli *redis.Client
}

func NewMessageProvider(cli *redis.Client) *MessageProvider {
	return &MessageProvider{cli: cli}
}

func (m *MessageProvider) GetMessagesForChatId(chatId int64) ([]models.MessageData, error) {
	data, err := m.cli.Get(getMessagesKey(chatId)).Result()

	if errors.Is(err, redis.Nil) {
		return nil, ErrorNotFound
	}

	if err != nil {
		return nil, err
	}

	var result []models.MessageData

	err = json.Unmarshal([]byte(data), &result)

	if err != nil {
		return nil, err
	}

	return result, nil
}

func (m *MessageProvider) SaveMessagesForChatId(chatId int64, messages []models.MessageData) error {
	data, err := json.Marshal(messages)
	if err != nil {
		return err
	}

	return m.cli.Set(getMessagesKey(chatId), data, 0).Err()
}
