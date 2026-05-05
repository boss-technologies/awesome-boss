package bosssocket

import (
    "log"
    "net"
    "strings"

    "github.com/gobwas/ws"
    "github.com/valyala/fasthttp"
)

// WebSocketHandlerFunc — тип функции обработчика соединения.
type WebSocketHandlerFunc func(conn net.Conn)

func WebSocketHandler(handler WebSocketHandlerFunc) fasthttp.RequestHandler {
    bs := &BossSocket{handler: handler}
    return bs.UpgradeHandler()
}

// BossSocket — внутренняя структура.
type BossSocket struct {
    handler WebSocketHandlerFunc
}

// UpgradeHandler возвращает fasthttp.RequestHandler.
func (bs *BossSocket) UpgradeHandler() fasthttp.RequestHandler {
    return func(ctx *fasthttp.RequestCtx) {
        if !isWebSocketUpgrade(ctx) {
            ctx.Error("Not a WebSocket upgrade request", fasthttp.StatusBadRequest)
            return
        }

        ctx.Hijack(func(conn net.Conn) {
            defer conn.Close()

            _, err := ws.Upgrade(conn)
            if err != nil {
                log.Printf("WebSocket upgrade error: %v", err)
                return
            }

            bs.handler(conn)
        })
    }
}

// isWebSocketUpgrade проверяет заголовки.
func isWebSocketUpgrade(ctx *fasthttp.RequestCtx) bool {
    connection := string(ctx.Request.Header.Peek("Connection"))
    if !strings.Contains(strings.ToLower(connection), "upgrade") {
        return false
    }
    upgrade := string(ctx.Request.Header.Peek("Upgrade"))
    return strings.Contains(strings.ToLower(upgrade), "websocket")
}

// Выполнено с любовью для Босса 🐈‍