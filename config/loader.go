package config

import (
    "fmt"
    "os"

    "github.com/joho/godotenv"
    "github.com/pelletier/go-toml/v2"
)

/* LoadConfig загружает конфигурацию из указанного файла.
Сначала загружает .env файл (если есть), затем применяет переменные окружения поверх. */
func LoadConfig(filePath string) (*BossConfig, error) {
    // 1. Загружаем .env файл, игнорируем ошибку если его нет (для разработки)
    _ = godotenv.Load() // по умолчанию ищет .env в текущей папке

    file, err := os.Open(filePath)
    if err != nil {
        return nil, fmt.Errorf("failed to open config file: %w", err)
    }
    defer file.Close()

    var cfg BossConfig
    decoder := toml.NewDecoder(file)
    if err := decoder.Decode(&cfg); err != nil {
        return nil, fmt.Errorf("failed to decode config: %w", err)
    }

    // 2. Применяем переменные окружения (они перекрывают значения из файла)
    applyEnvOverrides(&cfg)

    return &cfg, nil
}

// applyEnvOverrides заменяет поля конфига значениями из переменных окружения.
func applyEnvOverrides(cfg *BossConfig) {
    if key := os.Getenv("BOSS_AUTH_SECRET"); key != "" {
        cfg.Auth.SecretKey = key
    }
}

// Выполнено с любовью для Босса 🐈‍