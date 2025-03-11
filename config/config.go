package config

type AppConfig struct {
	TelegramConfig
	RedisConfig
	AiConfig
}

type TelegramConfig struct {
	Token          string
	AllowedChatIds []int64
}

type RedisConfig struct {
	Addr     string
	Password string
	DB       int
}

type AiConfig struct {
	Token string
}
