package fintech

import (
	"log/slog"
	"time"

	"github.com/boss-technologies/awesome-boss/core"
)

// Данное middleware является доказательством того, что middleware с пакетом fintech работают только в fintech-режиме.
// Audit возвращает middleware для логирования всех HTTP-запросов. Это только прототип аудита, в будущем аудит будет лучше

func Audit() core.Middleware {
    return func(next core.Handler) core.Handler {
        return func(ctx *core.BossContext) error {
            start := time.Now()

            // Логируем начало запроса (опционально)

            err := next(ctx)

            duration := time.Since(start)
            status := ctx.Response.StatusCode()

            // Структурированное логирование с нужными полями
            slog.Info("http request",
                "method", string(ctx.Method()),
                "path", string(ctx.Path()),
                "status", status,
                "duration_ms", duration.Milliseconds(),
                "ip", ctx.RemoteIP().String(),
                "user_agent", string(ctx.UserAgent()),
            )

            return err
        }
    }
}

// Выполнено с любовью для Босса 🐈‍