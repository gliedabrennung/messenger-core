package config

import (
	"sync"
	"time"

	"github.com/cloudwego/hertz/pkg/common/hlog"
	"github.com/ilyakaznacheev/cleanenv"
)

type Config struct {
	DSN       string        `env:"DSN" env-required:"true"`
	JWTSecret string        `env:"JWT_SECRET" env-required:"true"`
	JWTTTL    time.Duration `env:"JWT_TTL" env-default:"24h"`
}

var (
	instance *Config
	once     sync.Once
)

func GetConfig() *Config {
	once.Do(func() {
		instance = &Config{}
		if err := cleanenv.ReadConfig(".env", instance); err != nil {
			if envErr := cleanenv.ReadEnv(instance); envErr != nil {
				hlog.Error("config error: ", envErr)
			}
		}
	})
	return instance
}
