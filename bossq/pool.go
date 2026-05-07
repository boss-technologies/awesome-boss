package bossq

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// DefaultPoolConfig возвращает оптимальные настройки пула для FinTech-нагрузок.
// Эти значения подобраны для высокой конкурентности и минимальных задержек.
func DefaultPoolConfig(connString string) (*pgxpool.Config, error) {
	config, err := pgxpool.ParseConfig(connString)
	if err != nil {
		return nil, fmt.Errorf("bossq: неверный формат строки подключения: %w", err)
	}

	// Настройки по умолчанию
	config.MaxConns = 20                     // максимум открытых соединений
	config.MinConns = 2                      // всегда держим хотя бы 2 (быстрый старт)
	config.MaxConnLifetime = 30 * time.Minute // пересоздаём соединения не чаще чем раз в 30 минут
	config.MaxConnIdleTime = 5 * time.Minute  // если соединение не используется 5 минут, закрываем
	config.HealthCheckPeriod = 1 * time.Minute // проверяем здоровье соединений раз в минуту
	config.ConnConfig.ConnectTimeout = 3 * time.Second // таймаут при установке соединения

	// Для FinTech критично не терять данные при внезапных отключениях
	config.ConnConfig.RuntimeParams["application_name"] = "awesome_boss"
	config.ConnConfig.RuntimeParams["statement_timeout"] = "30s"

	return config, nil
}

// NewPool создаёт новый пул соединений с оптимальными настройками.
// connString — стандартная строка подключения PostgreSQL (например, "postgres://user:pass@localhost:5432/mydb?sslmode=disable").
func NewPool(ctx context.Context, connString string) (*pgxpool.Pool, error) {
	config, err := DefaultPoolConfig(connString)
	if err != nil {
		return nil, err
	}

	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("bossq: не удалось создать пул: %w", err)
	}

	// Проверяем, что пул действительно работает
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("bossq: пинг не прошёл, возможно, база данных недоступна: %w", err)
	}

	return pool, nil
}

// Выполнено с любовью для Босса 🐈‍