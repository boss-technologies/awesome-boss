// socket.go
package bosssocket

import (
    "log"
    "net"
    "strings"

    "github.com/gobwas/ws"
    "github.com/valyala/fasthttp"
)

type WebSocketHandlerFunc func(net.Conn)

var bufferPool = NewFixedBufferPool(4096, 65536) // Буферы от 4KB до 64KB

func WebSocketHandler(handler WebSocketHandlerFunc) fasthttp.RequestHandler {
    bs := &BossSocket{handler: handler}
    return bs.UpgradeHandler()
}

type BossSocket struct {
    handler WebSocketHandlerFunc
}

func (bs *BossSocket) UpgradeHandler() fasthttp.RequestHandler {
    return func(ctx *fasthttp.RequestCtx) {
        if !isWebSocketUpgrade(ctx) {
            ctx.Error("Not a WebSocket upgrade request", fasthttp.StatusBadRequest)
            return
        }

        ctx.Hijack(func(conn net.Conn) {
            defer conn.Close()

            if _, err := ws.Upgrade(conn); err != nil {
                log.Printf("WebSocket upgrade error: %v", err)
                return
            }

            bs.handler(conn)
        })
    }
}

func isWebSocketUpgrade(ctx *fasthttp.RequestCtx) bool {
    connection := string(ctx.Request.Header.Peek("Connection"))
    if !strings.Contains(strings.ToLower(connection), "upgrade") {
        return false
    }
    upgrade := string(ctx.Request.Header.Peek("Upgrade"))
    return strings.Contains(strings.ToLower(upgrade), "websocket")
}

// Выполнено с любовью для Босса 🐈‍