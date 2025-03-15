package storage

import (
	"errors"
	"fmt"
)

var (
	ErrorNotFound = errors.New("not found")
)

func getTicketQueueKey() string {
	return "tickets:queue"
}

func getTicketPoolKey(chatId int64) string {
	return fmt.Sprintf("tickets:pool:%d", chatId)
}

func getTicketKey(ticketId string) string {
	return fmt.Sprintf("tickets:%s", ticketId)
}

func getMessagesKey(chatId int64) string {
	return fmt.Sprintf("messages:%d", chatId)
}

func getMessagesNonceKey(chatId int64) string {
	return fmt.Sprintf("messages:nonce:%d", chatId)
}
