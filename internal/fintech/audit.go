package fintech

import (
	"crypto/rand"
	"encoding/hex"
	"time"

	"github.com/boss-technologies/awesome-boss/core"
	"github.com/boss-technologies/awesome-boss/internal/audit"
)

var auditWriter audit.Writer // глобальная переменная, инициализируется в main

// SetAuditWriter позволяет установить writer для аудита (например, AsyncWriter)
func SetAuditWriter(w audit.Writer) {
	auditWriter = w
}

// AuditMiddleware — middleware для записи аудита каждого запроса
func AuditMiddleware(next core.Handler) core.Handler {
	return func(ctx *core.BossContext) error {
		start := time.Now()
		err := next(ctx)

		// Если аудит не настроен — пропускаем
		if auditWriter == nil {
			return err
		}

		userID, _ := ctx.GetUserID()
        entry := audit.Entry[string]{
            Timestamp:  start,
            RequestID:  generateRequestID(),
            Method:     string(ctx.Method()),
            Path:       string(ctx.Path()),
            StatusCode: ctx.Response.StatusCode(),
            Duration:   time.Since(start).Milliseconds(),
            UserID:     userID,
            RemoteIP:   ctx.RemoteIP().String(),
            UserAgent:  string(ctx.UserAgent()),
        }

		// Отправляем в writer (асинхронно, если writer асинхронный)
		_ = auditWriter.Write(entry)
		return err
	}
}

func generateRequestID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// Выполнено с любовью для Босса 🐈‍