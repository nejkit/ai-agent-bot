package storage

import (
	"errors"
	"fmt"
)

var (
	ErrorNotFound = errors.New("not found")
)

func getTicketPoolKey(chatId int64) string {
	return fmt.Sprintf("tickets:pool:%d", chatId)
}

func getTicketKey(ticketId string) string {
	return fmt.Sprintf("tickets:%s", ticketId)
}

func getMessagesKey(chatId string) string {
	return fmt.Sprintf("messages:%s", chatId)
}

func getChatsKey() string {
	return fmt.Sprintf("chats")
}

func getSuperGroupConfigsKey(chatID int64) string {
	return fmt.Sprintf("supergroup:config:%d", chatID)
}
