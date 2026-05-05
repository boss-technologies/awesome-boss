package fintech

import (
	"github.com/boss-technologies/awesome-boss/core"
)

func BodyLimit(maxBytes int) core.Middleware {
    return func(next core.Handler) core.Handler {
        return func(ctx *core.BossContext) error {
            if ctx.Request.Header.ContentLength() > maxBytes {
                ctx.Response.SetStatusCode(413)
                ctx.Response.SetBodyString("Request Entity Too Large")
                return nil
            }
            return next(ctx)
        }
    }
}

// Выполнено с любовью для Босса 🐈‍