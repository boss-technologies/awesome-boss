package fintech

import (
	"strings"
	"github.com/boss-technologies/awesome-boss/core" 
	"github.com/boss-technologies/awesome-boss/auth" 
)

func AuthOptional(secretKeyHex string) core.Middleware {
    return func(next core.Handler) core.Handler {
        return func(ctx *core.BossContext) error {
            authHeader := string(ctx.Request.Header.Peek("Authorization"))
            if authHeader == "" {
                return next(ctx) // без токена — ок
            }
            parts := strings.Split(authHeader, " ")
            if len(parts) == 2 && parts[0] == "Bearer" {
                userID, err := auth.ValidateToken(secretKeyHex, parts[1])
                if err == nil {
                    ctx.User = userID // пользователь опознан
                }
            }
            return next(ctx)
        }
    }
}

// Выполнено с любовью для Босса 🐈‍