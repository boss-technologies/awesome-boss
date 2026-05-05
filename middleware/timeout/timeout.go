package timeout
import (
	"context"
	"time"
	"github.com/boss-technologies/awesome-boss/core"
)

func Timeout(timeout time.Duration) core.Middleware {
    return func(next core.Handler) core.Handler {
        return func(ctx *core.BossContext) error {
            ctxWithTimeout, cancel := context.WithTimeout(context.Background(), timeout)
            defer cancel()
            // Используем канал для синхронизации
            done := make(chan error, 1)
            go func() { done <- next(ctx) }()
            select {
            case err := <-done:
                return err
            case <-ctxWithTimeout.Done():
                ctx.Response.SetStatusCode(504)
                ctx.Response.SetBodyString("Request Timeout")
                return nil
            }
        }
    }
}

// Выполнено с любовью для Босса 🐈‍