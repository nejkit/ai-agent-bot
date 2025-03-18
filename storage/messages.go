package storage

import (
	"encoding/json"
	"errors"
	"github.com/go-redis/redis"
	"github.com/nejkit/ai-agent-bot/models"
	"strconv"
)

type MessageProvider struct {
	cli *redis.Client
}

func NewMessageProvider(cli *redis.Client) *MessageProvider {
	return &MessageProvider{cli: cli}
}

func (m *MessageProvider) GetMessagesForChatId(chatId string) ([]models.MessageData, error) {
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

func (m *MessageProvider) SaveMessagesForChatId(chatId string, messages []models.MessageData) error {
	data, err := json.Marshal(messages)
	if err != nil {
		return err
	}

	return m.cli.Set(getMessagesKey(chatId), data, 0).Err()
}

func (m *MessageProvider) SaveChatToAllowed(chatId int64) error {
	return m.cli.SAdd(getChatsKey(), chatId).Err()
}

func (m *MessageProvider) GetAllowedChats() []int64 {
	data, err := m.cli.SMembers(getChatsKey()).Result()

	if errors.Is(err, redis.Nil) {
		return []int64{}
	}

	if err != nil {
		return []int64{}
	}

	parsedData := make([]int64, len(data))

	for i := range data {
		parsedData[i], _ = strconv.ParseInt(data[i], 10, 64)
	}

	return parsedData
}

func (m *MessageProvider) DeleteChatFromAllowed(chatId int64) error {
	return m.cli.SRem(getChatsKey(), chatId).Err()
}

func (m *MessageProvider) SaveSettingsForSuperGroupChat(chatId int64, info *models.SuperGroupConfigModel) error {
	data, err := json.Marshal(info)

	if err != nil {
		return err
	}

	return m.cli.Set(getSuperGroupConfigsKey(chatId), data, 0).Err()
}

func (m *MessageProvider) GetSettingsForSuperGroupChat(chatId int64) (*models.SuperGroupConfigModel, error) {
	data, err := m.cli.Get(getSuperGroupConfigsKey(chatId)).Result()

	if errors.Is(err, redis.Nil) {
		return nil, ErrorNotFound
	}

	if err != nil {
		return nil, err
	}

	var result models.SuperGroupConfigModel
	err = json.Unmarshal([]byte(data), &result)
	if err != nil {
		return nil, err
	}

	return &result, nil
}
