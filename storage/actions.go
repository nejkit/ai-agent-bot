package storage

import (
	"errors"
	"fmt"
	"github.com/go-redis/redis"
	"strconv"
)

type ActionProvider struct {
	cli *redis.Client
}

func NewActionProvider(cli *redis.Client) *ActionProvider {
	return &ActionProvider{cli: cli}
}

type Action int

const (
	ActionTypeNone Action = iota + 1
	ActionTypeAddUser
	ActionTypeCheckFile
	ActionTypeDeleteUser
)

func (a *ActionProvider) SaveAction(chatId int64, action Action) error {
	return a.cli.Set(fmt.Sprintf("action:%d", chatId), int(action), 0).Err()
}

func (a *ActionProvider) LoadAction(chatId int64) (Action, error) {
	result, err := a.cli.Get(fmt.Sprintf("action:%d", chatId)).Result()

	if errors.Is(err, redis.Nil) {
		return 1, nil
	}

	if err != nil {
		return 1, err
	}

	parsed, err := strconv.ParseInt(result, 10, 32)

	if err != nil {
		return 1, err
	}

	return Action(parsed), nil
}
