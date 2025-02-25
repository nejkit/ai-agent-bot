package models

type UserModel struct {
	Username  string   `json:"username"`
	ChatId    int64    `json:"chatId"`
	UserId    int64    `json:"userId"`
	OldHashes []string `json:"oldHash"`
}
