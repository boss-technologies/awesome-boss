package fintech

import (
    "strings"

    "github.com/boss-technologies/awesome-boss/core"          // правильный импорт
    "github.com/boss-technologies/awesome-boss/internal/auth" // внутренний пакет
)

// RequireAuth возвращает middleware, требующее валидный BAT-токен.
// secretKeyHex – 32-байтовый ключ в hex-формате.
func RequireAuth(secretKeyHex string) core.Middleware {
    return func(next core.Handler) core.Handler {
        return func(ctx *core.BossContext) error {
            authHeader := string(ctx.Request.Header.Peek("Authorization"))
            if authHeader == "" {
                return ctx.JSON(401, map[string]string{"error": "missing token"})
            }

            parts := strings.Split(authHeader, " ")
            if len(parts) != 2 || parts[0] != "Bearer" {
                return ctx.JSON(401, map[string]string{"error": "invalid auth header"})
            }

            userID, err := auth.ValidateToken(secretKeyHex, parts[1])
            if err != nil {
                return ctx.JSON(401, map[string]string{"error": "invalid token"})
            }

            ctx.User = userID
            return next(ctx)
        }
    }
}

// Выполнено с любовью для Босса 🐈‍