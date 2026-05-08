package ddos

import (
	"sync"
	"time"

	"github.com/boss-technologies/awesome-boss/core"
	"golang.org/x/time/rate"
)

// RateLimiterConfig настройки ограничения частоты
type RateLimiterConfig struct {
	RequestsPerSecond int           // запросов в секунду
	Burst             int           // размер всплеска
	CleanupInterval   time.Duration // как часто удалять неактивные IP
}

// ConcurrencyLimiterConfig настройки ограничения параллельности
type ConcurrencyLimiterConfig struct {
	MaxConcurrent int // максимальное число одновременно обрабатываемых запросов
}

// IPRateLimiter хранит лимитеры для каждого IP
type IPRateLimiter struct {
	limiters sync.Map
	cfg      RateLimiterConfig
}

// NewIPRateLimiter создаёт новый лимитер
func NewIPRateLimiter(cfg RateLimiterConfig) *IPRateLimiter {
	lim := &IPRateLimiter{cfg: cfg}
	if cfg.CleanupInterval > 0 {
		go lim.cleanupLoop()
	}
	return lim
}

// getLimiter возвращает лимитер для IP
func (l *IPRateLimiter) getLimiter(ip string) *rate.Limiter {
	lim := rate.NewLimiter(rate.Limit(l.cfg.RequestsPerSecond), l.cfg.Burst)
	actual, _ := l.limiters.LoadOrStore(ip, lim)
	return actual.(*rate.Limiter)
}

// Allow проверяет, разрешён ли запрос с данного IP
func (l *IPRateLimiter) Allow(ip string) bool {
	return l.getLimiter(ip).Allow()
}

// cleanupLoop удаляет неактивные лимитеры раз в CleanupInterval
func (l *IPRateLimiter) cleanupLoop() {
	ticker := time.NewTicker(l.cfg.CleanupInterval)
	defer ticker.Stop()
	for range ticker.C {
		l.limiters.Range(func(key, value any) bool {
			lim := value.(*rate.Limiter)
			// Если лимитер не использовался последние CleanupInterval, удаляем
			// Для простоты удаляем все, у которых резерв (burst) полон.
			// Более точная проверка потребовала бы хранения времени последнего использования.
			if lim.Burst() == l.cfg.Burst && lim.Tokens() == float64(l.cfg.Burst) {
				l.limiters.Delete(key)
			}
			return true
		})
	}
}

// ConcurrencyLimiter ограничивает число параллельных запросов
type ConcurrencyLimiter struct {
	sem chan struct{}
}

// NewConcurrencyLimiter создаёт лимитер параллельности
func NewConcurrencyLimiter(max int) *ConcurrencyLimiter {
	return &ConcurrencyLimiter{
		sem: make(chan struct{}, max),
	}
}

// Acquire захватывает слот, возвращает функцию Release
func (c *ConcurrencyLimiter) Acquire() (release func(), ok bool) {
	select {
	case c.sem <- struct{}{}:
		return func() { <-c.sem }, true
	default:
		return nil, false
	}
}

// DDOSMiddleware объединяет rate limiting и concurrency limiting
func DDOSMiddleware(rateCfg RateLimiterConfig, concCfg ConcurrencyLimiterConfig) core.Middleware {
	rateLim := NewIPRateLimiter(rateCfg)
	concLim := NewConcurrencyLimiter(concCfg.MaxConcurrent)

	return func(next core.Handler) core.Handler {
		return func(ctx *core.BossContext) error {
			// 1. Rate limit по IP
			ip := ctx.RemoteIP().String()
			if !rateLim.Allow(ip) {
				ctx.Response.SetStatusCode(429)
				ctx.Response.SetBodyString("Too Many Requests")
				return nil
			}

			// 2. Concurrency limit
			release, ok := concLim.Acquire()
			if !ok {
				ctx.Response.SetStatusCode(503)
				ctx.Response.SetBodyString("Server Busy")
				return nil
			}
			defer release()

			// 3. Передаём управление дальше
			return next(ctx)
		}
	}
}

// Выполнено с любовью для Босса 🐈‍