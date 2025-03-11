package storage

import (
	"encoding/json"
	"errors"
	"github.com/go-redis/redis"
	"github.com/nejkit/ai-agent-bot/models"
)

type TicketProvider struct {
	cli *redis.Client
}

func NewTicketProvider(cli *redis.Client) *TicketProvider {
	return &TicketProvider{cli: cli}
}

func (t *TicketProvider) GetTicketIdForProcess() (string, error) {
	ticketId, err := t.cli.LPop(getTicketQueueKey()).Result()

	if errors.Is(err, redis.Nil) {
		return "", ErrorNotFound
	}

	if err != nil {
		return "", err
	}

	return ticketId, nil
}

func (t *TicketProvider) StoreTicketIntoPool(chatId int64, ticketId string, requestMessageId int) error {
	return t.cli.ZAdd(getTicketPoolKey(chatId), redis.Z{
		Score:  float64(requestMessageId),
		Member: ticketId,
	}).Err()
}

func (t *TicketProvider) GetTicketFromPool(chatId int64) (string, error) {
	result, err := t.cli.ZRangeByScore(getTicketPoolKey(chatId), redis.ZRangeBy{Min: "-inf", Max: "+inf", Count: 1}).Result()

	if errors.Is(err, redis.Nil) {
		return "", ErrorNotFound
	}

	if err != nil {
		return "", err
	}

	if len(result) == 0 {
		return "", ErrorNotFound
	}

	return result[0], t.cli.ZRem(getTicketPoolKey(chatId), result[0]).Err()
}

func (t *TicketProvider) GetMinimumScoreFromPool(chatId int64) (int, error) {
	result, err := t.cli.ZRangeByScoreWithScores(getTicketPoolKey(chatId), redis.ZRangeBy{Min: "-inf", Max: "+inf", Count: 1}).Result()

	if errors.Is(err, redis.Nil) {
		return 0, ErrorNotFound
	}

	if err != nil {
		return 0, err
	}

	if len(result) == 0 {
		return 0, ErrorNotFound
	}

	return int(result[0].Score), nil
}

func (t *TicketProvider) GetTicketById(ticketId string) (*models.ExternalChatTicketData, error) {
	ticketData, err := t.cli.Get(getTicketKey(ticketId)).Result()

	if errors.Is(err, redis.Nil) {
		return nil, ErrorNotFound
	}

	if err != nil {
		return nil, err
	}

	var ticketModel models.ExternalChatTicketData

	err = json.Unmarshal([]byte(ticketData), &ticketModel)

	if err != nil {
		return nil, err
	}

	return &ticketModel, nil
}

func (t *TicketProvider) SaveTicket(data *models.ExternalChatTicketData) error {
	ticketData, err := json.Marshal(data)

	if err != nil {
		return err
	}

	if err = t.cli.Set(getTicketKey(data.Id), ticketData, 0).Err(); err != nil {
		return err
	}

	if err = t.cli.RPush(getTicketQueueKey(), data.Id).Err(); err != nil {
		return err
	}

	return nil
}

func (t *TicketProvider) DeleteTicket(ticketId string) error {
	return t.cli.Del(getTicketKey(ticketId)).Err()
}

func (t *TicketProvider) PushTicketIdToProcess(ticketId string) error {
	return t.cli.RPush(getTicketQueueKey(), ticketId).Err()
}
