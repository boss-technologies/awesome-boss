package config

import (
	"fmt"
	"strings"
	"time"
)

type Mode string

const (
    ModeNormal  Mode = "normal"
    ModeFintech Mode = "fintech"
)

// BossConfig – главная структура конфигурации
type BossConfig struct {
    Server   ServerConfig   `toml:"server"`
    Auth     AuthConfig     `toml:"auth"`
    Database DatabaseConfig `toml:"database"`
    Logging  LoggingConfig  `toml:"logging"`
    Mode     Mode           `toml:"mode"`
}

type ServerConfig struct {
    Port          int           `toml:"port"`
    ReadTimeout   time.Duration `toml:"read_timeout"`
    WriteTimeout  time.Duration `toml:"write_timeout"`
    EnableGzip    bool          `toml:"enable_gzip"`
}

type AuthConfig struct {
    SecretKey string `toml:"secret_key"`
}

type DatabaseConfig struct {
    Url string `toml:"url_database"`
}

type LoggingConfig struct {
    Level string `toml:"level"`
}

// Default возвращает конфигурацию по умолчанию
func Default() *BossConfig {
    return &BossConfig{
        Mode: ModeNormal,
        Server: ServerConfig{
            Port:         8080,
            ReadTimeout:  30 * time.Second,
            WriteTimeout: 30 * time.Second,
            EnableGzip:   true,
        },
        Auth: AuthConfig{
            SecretKey: "", // должен быть сгенерирован отдельно
        },
        Database: DatabaseConfig{
            Url: "postgres://postgres:root@localhost:5432/dbname?sslmode=disable",
        },
        Logging: LoggingConfig{
            Level: "info",
        },
    }
}

func (cfg *BossConfig) Validate() error {
    if cfg.Mode == ModeFintech {
        // Простая проверка: в fintech-режим нельзя входить с sslmode=disable
        if strings.Contains(cfg.Database.Url, "sslmode=disable") {
            return fmt.Errorf("FINANCIAL MODE: insecure database connection is forbidden, use sslmode=verify-full")
        }
    }
    return nil
}

// Выполнено с любовью для Босса 🐈‍