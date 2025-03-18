package config

type AppConfig struct {
	TelegramConfig `envPrefix:"TELEGRAM_"`
	RedisConfig    `envPrefix:"REDIS_"`
	AiConfig       `envPrefix:"AI_"`
}

type TelegramConfig struct {
	Token        string  `env:"TOKEN"`
	AllowedUsers []int64 `env:"ALLOWED_USERS" envSeparator:","`
}

type RedisConfig struct {
	Addr     string `env:"ADDR"`
	Port     int    `env:"PORT"`
	Password string `env:"PASSWORD" envDefault:""`
	DB       int    `env:"DB" envDefault:"0"`
}

type AiConfig struct {
	Token string `env:"TOKEN"`
}
