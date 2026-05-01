package core

import (
	"encoding/json"
	"github.com/valyala/fasthttp"
	"maps"
	"sync"
    "github.com/mailru/easyjson"
)

// fastMarshal пытается использовать easyjson, если тип поддерживает,
// иначе падает на стандартный json.Marshal
func fastMarshal(v any) ([]byte, error) {
    // Проверяем, реализует ли v easyjson.Marshaler
    if m, ok := v.(easyjson.Marshaler); ok {
        return easyjson.Marshal(m)
    }
    // fallback к стандартной библиотеке
    return json.Marshal(v)
}

// fastUnmarshal аналогично для декодирования
func fastUnmarshal(data []byte, v any) error {
    // Проверяем, реализует ли v easyjson.Unmarshaler
    if u, ok := v.(easyjson.Unmarshaler); ok {
        return easyjson.Unmarshal(data, u)
    }
    return json.Unmarshal(data, v)
}

// BossContext — контекст запроса. User пока interface{} (будет типизирован через утилиты)
type BossContext struct {
	*fasthttp.RequestCtx
	User  any
	Store map[string]any
	mu    sync.RWMutex
}

// NewBossContext создаёт новый контекст с инициализированным хранилищем.
func NewBossContext(ctx *fasthttp.RequestCtx) *BossContext {
	return &BossContext{
		RequestCtx: ctx,
		Store:      make(map[string]any),
	}
}

// Set сохраняет значение в контексте запроса (потокобезопасно).
func (c *BossContext) Set(key string, value any) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.Store[key] = value
}

// Get возвращает значение из контекста запроса.
func (c *BossContext) Get(key string) any {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.Store[key]
}

// GetAll возвращает копию всех зависимостей (для внутреннего использования).
func (c *BossContext) GetAll() map[string]any {
	c.mu.RLock()
	defer c.mu.RUnlock()
	copy := make(map[string]any, len(c.Store))
	maps.Copy(copy, c.Store)
	return copy
}

func (c *BossContext) Method() string {
	return string(c.Request.Header.Method())
}

func (c *BossContext) Path() string {
	return string(c.Request.URI().Path())
}

// Handler определяет сигнатуру обработчика маршрута
type Handler func(*BossContext) error

// Middleware позволяет модифицировать поведение обработчиков
type Middleware func(Handler) Handler

// JSON сериализует v в JSON и отправляет ответ (теперь с fastMarshal)
func (ctx *BossContext) JSON(statusCode int, v any) error {
    data, err := fastMarshal(v)
    if err != nil {
        return err
    }
    ctx.Response.Header.SetContentType("application/json")
    ctx.Response.SetStatusCode(statusCode)
    ctx.Response.SetBody(data)
    return nil
}



func (ctx *BossContext) WriteString(s string) {
	ctx.Response.SetBodyString(s)
}

func (ctx *BossContext) Param(key string) string {
	val := ctx.UserValue(key)
	if val == nil {
		return ""
	}
	s, ok := val.(string)
	if !ok {
		return ""
	}
	return s
}

// GetTyped возвращает значение из хранилища с проверкой типа
func GetTyped[T any](c *BossContext, key string) (T, bool) {
	v := c.Get(key)
	t, ok := v.(T)
	return t, ok
}

// SetTyped сохраняет значение с конкретным типом
func SetTyped[T any](c *BossContext, key string, val T) {
	c.Set(key, val)
}

// BindJSON декодирует тело запроса в указанный тип (теперь с fastUnmarshal)
func BindJSON[T any](c *BossContext) (T, error) {
    var result T
    err := fastUnmarshal(c.Request.Body(), &result)
    return result, err
}


// UserTyped возвращает User как конкретный тип
func UserTyped[T any](c *BossContext) (T, bool) {
	u, ok := c.User.(T)
	return u, ok
}

// Выполнено с любовью для Босса 🐈
