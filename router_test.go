package awesomeboss

import (
    "testing"
    "github.com/boss-technologies/awesome-boss/core"
    "github.com/valyala/fasthttp"
)

// Бенчмарк для простого обработчика
func BenchmarkSimpleHandler(b *testing.B) {
    app := New(nil)
    app.Get("/ping", func(ctx *core.BossContext) error {
        ctx.WriteString("pong")
        return nil
    })
	
    // Создаём тестовый контекст запроса (не входит в замер времени!)
    ctx := &fasthttp.RequestCtx{}
    ctx.Request.Header.SetMethod("GET")
    ctx.Request.SetRequestURI("/ping")

    // СБРОС ТАЙМЕРА! Убираем время на подготовку из замера
    b.ResetTimer()

    // Основной цикл бенчмарка
    for i := 0; i < b.N; i++ {
        app.router.Handler(ctx)
    }
}

// Выполнено с любовью для Босса 🐈‍