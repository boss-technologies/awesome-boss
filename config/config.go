package config

import (
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
    EnableHTTP2   bool          `toml:"enable_http2"`
}

type AuthConfig struct {
    SecretKey string `toml:"secret_key"`
}

type DatabaseConfig struct {
    Host     string `toml:"host"`
    Port     int    `toml:"port"`
    User     string `toml:"user"`
    Password string `toml:"password"`
    Name     string `toml:"name"`
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
            Host: "localhost",
            Port: 5432,
        },
        Logging: LoggingConfig{
            Level: "info",
        },
    }
}

// Выполнено с любовью для Босса 🐈‍