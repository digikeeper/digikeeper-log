package main

import (
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/ilyakaznacheev/cleanenv"
)

type CommonConfig struct {
	EnvName string `env:"ENVIRONMENT_NAME" env-default:"dev"`
}

type APIConfig struct {
	Timeout   time.Duration `env:"TIMEOUT" env-default:"5s"`
	LocalPort string        `env:"LOCAL_PORT" env-default:"9000"`
	LocalHost string        `env:"LOCAL_HOST" env-default:"localhost"`
}

type LogStorageConfig struct {
	Path string `env:"PATH" env-required:"true"`
}

type SQLiteConfig struct {
	JournalMode string        `env:"JOURNAL_MODE" env-default:"WAL"`
	BusyTimeout time.Duration `env:"BUSY_TIMEOUT" env-default:"5s"`
}

type Config struct {
	Common     CommonConfig     `yaml:"common"`
	LogStorage LogStorageConfig `yaml:"log_storage" env-prefix:"LOG_STORAGE_"`
	SQLite     SQLiteConfig     `yaml:"sqlite" env-prefix:"SQLITE_"`
	API        APIConfig        `yaml:"api" env-prefix:"API_"`
}

func (c *Config) IsDevEnv() bool {
	return strings.HasPrefix(strings.ToLower(c.Common.EnvName), "dev")
}

func configure() Config {
	var cfg Config

	if err := cleanenv.ReadConfig(".env", &cfg); err != nil {
		log.Printf("No .env loaded: %v", err)
	}

	if err := cleanenv.ReadEnv(&cfg); err != nil {
		panic(fmt.Sprintf("failed to fetch env vars: %v", err))
	}

	return cfg
}
