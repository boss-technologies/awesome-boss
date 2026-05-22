package bosssocket

import (
    "context"
    "log"
    "net"
    "strings"
    "sync"
    "time"

    "github.com/gobwas/ws"
    "github.com/gobwas/ws/wsutil"
    "github.com/valyala/fasthttp"
)

// Conn – обёртка над WebSocket-соединением.
type Conn struct {
    net.Conn
    bufferPool *FixedBufferPool
    mu         sync.Mutex
}

// ReadText читает следующее текстовое сообщение.
// Использует ReadClientData, которая возвращает 3 значения.
func (c *Conn) ReadText() (string, error) {
    buf := c.bufferPool.Get()
    defer c.bufferPool.Put(buf)

    // Читаем данные в наш буфер. ReadClientData возвращает срез байт, код операции и ошибку.
    data, _, err := wsutil.ReadClientData(c.Conn)
    if err != nil {
        return "", err
    }
    return string(data), nil
}

// WriteText отправляет текстовое сообщение.
// Использует WriteServerText для отправки сообщений от сервера.
func (c *Conn) WriteText(msg string) error {
    c.mu.Lock()
    defer c.mu.Unlock()
    return wsutil.WriteServerText(c.Conn, []byte(msg))
}

// Close отправляет close frame и закрывает соединение.
func (c *Conn) Close() error {
    // Отправляем close frame с нормальным статусом закрытия.
    // WriteMessage — это функция, которая записывает сообщение с заданным кодом операции.
    // Используем ws.OpClose для отправки close frame.
    err := wsutil.WriteMessage(c.Conn, ws.StateServerSide, ws.OpClose, nil)
    if err != nil {
        _ = c.Conn.Close()
        return err
    }
    // Закрываем соединение после отправки фрейма.
    return c.Conn.Close()
}

// ------------------------------------------------------------

// HandlerFunc – пользовательская функция для обработки WebSocket.
type HandlerFunc func(ctx context.Context, conn *Conn)

// WebSocketHandler возвращает fasthttp.RequestHandler с пулом буферов.
func WebSocketHandler(handler HandlerFunc) fasthttp.RequestHandler {
    pool := NewFixedBufferPool(4096, 65536) // Буфер 4KB, максимум 64KB
    return func(fctx *fasthttp.RequestCtx) {
        if !isWebSocketUpgrade(fctx) {
            fctx.Error("Not a WebSocket upgrade request", fasthttp.StatusBadRequest)
            return
        }

        fctx.Hijack(func(rawConn net.Conn) {
            // Апгрейд соединения до WebSocket.
            _, err := ws.Upgrade(rawConn)
            if err != nil {
                log.Printf("WebSocket upgrade error: %v", err)
                rawConn.Close()
                return
            }

            conn := &Conn{
                Conn:       rawConn,
                bufferPool: pool,
            }

            // Создаём контекст для управления временем жизни соединения.
            ctx, cancel := context.WithCancel(context.Background())
            defer cancel()

            // Пинг-понг: раз в 30 секунд отправляем пинг.
            go func() {
                ticker := time.NewTicker(30 * time.Second)
                defer ticker.Stop()
                for {
                    select {
                    case <-ctx.Done():
                        return
                    case <-ticker.C:
                        // Отправляем пинг-сообщение через WriteMessage.
                        if err := wsutil.WriteMessage(conn, ws.StateServerSide, ws.OpPing, nil); err != nil {
                            conn.Close()
                            return
                        }
                    }
                }
            }()

            // Вызов пользовательского хендлера.
            handler(ctx, conn)
        })
    }
}

// isWebSocketUpgrade проверяет заголовки Upgrade.
func isWebSocketUpgrade(ctx *fasthttp.RequestCtx) bool {
    connection := string(ctx.Request.Header.Peek("Connection"))
    if !strings.Contains(strings.ToLower(connection), "upgrade") {
        return false
    }
    upgrade := string(ctx.Request.Header.Peek("Upgrade"))
    return strings.EqualFold(upgrade, "websocket")
}

// Выполнено с любовью для Босса 🐈‍