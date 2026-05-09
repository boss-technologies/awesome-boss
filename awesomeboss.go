package awesomeboss

import (
	"context"
	"fmt"
	"log"
	"maps"
	"net"
	"os"
	"os/signal"
	"runtime/debug"
	"crypto/tls"
	"sync"
	"syscall"
	"time"
	"io/fs"

	"github.com/boss-technologies/awesome-boss/config"
	"github.com/boss-technologies/awesome-boss/core"
	"github.com/boss-technologies/awesome-boss/internal/csrf"
	"github.com/boss-technologies/awesome-boss/internal/fintech"
	"github.com/boss-technologies/awesome-boss/internal/socket"
	"github.com/boss-technologies/awesome-boss/middleware/ddos"

	"github.com/fasthttp/router"
	"github.com/valyala/fasthttp"
	"github.com/dgrr/http2"
)

// BossApp представляет HTTP-приложение с поддержкой middleware и групп маршрутов
type BossApp struct {
	router       *router.Router
	config       *config.BossConfig
	dependencies map[string]any // глобальные зависимости
	middlewares  []core.Middleware
}

// Group представляет группу маршрутов с общими middleware
type Group struct {
	app         *BossApp
	middlewares []core.Middleware
	prefix      string
}

// New создаёт новый экземпляр приложения
func New(cfg *config.BossConfig) *BossApp {
	if cfg == nil {
		cfg = config.Default()
	}
	app := &BossApp{
		router:       router.New(),
		config:       cfg,
		dependencies: make(map[string]any),
	}
	if cfg.Mode == config.ModeFintech {
		app.Use(fintech.AuditMiddleware)
		app.Use(ddos.DDOSMiddleware(
        ddos.RateLimiterConfig{
            RequestsPerSecond: 1000,
            Burst:             2000,
            CleanupInterval:   5 * time.Minute,
        },
        ddos.ConcurrencyLimiterConfig{
            MaxConcurrent: 10000,
        },
    	))
		app.Use(fintech.AuthOptional(cfg.Auth.SecretKey))
        app.Use(fintech.BodyLimit(1 << 20))
	}
	return app
}

// Set регистрирует глобальную зависимость.
func (app *BossApp) Set(key string, value any) {
	app.dependencies[key] = value
}

// Use добавляет глобальное middleware
func (app *BossApp) Use(mw core.Middleware) {
	app.middlewares = append(app.middlewares, mw)
}

// Get добавляет GET маршрут в корневое приложение
func (app *BossApp) Get(path string, handler core.Handler) {
	app.Group().Get(path, handler)
}

// Post добавляет POST маршрут в корневое приложение
func (app *BossApp) Post(path string, handler core.Handler) {
	app.Group().Post(path, handler)
}

// Put добавляет PUT маршрут в корневое приложение
func (app *BossApp) Put(path string, handler core.Handler) {
	app.Group().Put(path, handler)
}

// Delete добавляет DELETE маршрут в корневое приложение
func (app *BossApp) Delete(path string, handler core.Handler) {
	app.Group().Delete(path, handler)
}

// WebSocket регистрирует маршрут для WebSocket-соединений.
func (app *BossApp) WebSocket(path string, handler func(conn net.Conn)) {
	// Используем готовый обработчик из нашего пакета bosssocket
	wsHandler := bosssocket.WebSocketHandler(handler)
	app.router.GET(path, wsHandler)
}

// Group создаёт новую группу маршрутов
func (app *BossApp) Group(prefix ...string) *Group {
	p := ""
	if len(prefix) > 0 {
		p = prefix[0]
	}
	return &Group{app: app, middlewares: nil, prefix: p}
}

// Use добавляет middleware в группу
func (g *Group) Use(mw core.Middleware) *Group {
	g.middlewares = append(g.middlewares, mw)
	return g
}

// Get добавляет GET маршрут в группу
func (g *Group) Get(path string, handler core.Handler) *Group {
	g.addRoute("GET", path, handler)
	return g
}

// Post добавляет POST маршрут в группу
func (g *Group) Post(path string, handler core.Handler) *Group {
	g.addRoute("POST", path, handler)
	return g
}

// Put добавляет PUT маршрут в группу
func (g *Group) Put(path string, handler core.Handler) *Group {
	g.addRoute("PUT", path, handler)
	return g
}

// Delete добавляет DELETE маршрут в группу
func (g *Group) Delete(path string, handler core.Handler) *Group {
	g.addRoute("DELETE", path, handler)
	return g
}

// WebSocket регистрирует маршрут для WebSocket-соединений в группу
func (g *Group) WebSocket(path string, handler func(net.Conn)) *Group {
	fullPath := g.prefix + path
	wsHandler := bosssocket.WebSocketHandler(handler)
	g.app.router.GET(fullPath, wsHandler)
	return g
}

// addRoute добавляет маршрут с объединёнными middleware (глобальные + групповые)
func (g *Group) addRoute(method, path string, handler core.Handler) {
	fullPath := g.prefix + path

	// Сначала глобальные middleware, потом групповые
	chain := handler
	for i := len(g.app.middlewares) - 1; i >= 0; i-- {
		chain = g.app.middlewares[i](chain)
	}
	for i := len(g.middlewares) - 1; i >= 0; i-- {
		chain = g.middlewares[i](chain)
	}
	wrapped := g.app.wrapHandler(chain)

	switch method {
	case "GET":
		g.app.router.GET(fullPath, wrapped)
	case "POST":
		g.app.router.POST(fullPath, wrapped)
	case "PUT":
		g.app.router.PUT(fullPath, wrapped)
	case "DELETE":
		g.app.router.DELETE(fullPath, wrapped)
	}
}

// Logger — middleware для логирования запросов
func Logger(next core.Handler) core.Handler {
	return func(ctx *core.BossContext) error {
		start := time.Now()
		err := next(ctx)
		log.Printf("%s %s %d %v", ctx.Method(), ctx.Path(), ctx.Response.StatusCode(), time.Since(start))
		return err
	}
}

// EnableCSRF включает защиту от CSRF. secretKey — 32-байтовый ключ.
func (app *BossApp) EnableCSRF(secretKey []byte) {
	// Сначала генерируем и устанавливаем токен для безопасных методов
	app.Use(csrf.SetCSRFTokenMiddleware(secretKey))
	// Затем проверяем токен для всех остальных
	app.Use(csrf.CSRFMiddleware(secretKey))
}

// Пул для переиспользования BossContext
var bossCtxPool = sync.Pool{
	New: func() any {
		return &core.BossContext{
			Store: make(map[string]any), // инициализируем map
		}
	},
}

// wrapHandler оборачивает Handler в fasthttp.RequestHandler с пулом контекстов
func (app *BossApp) wrapHandler(chain core.Handler) fasthttp.RequestHandler {
	return func(fctx *fasthttp.RequestCtx) {
		// При получении из пула:
		bossCtx := bossCtxPool.Get().(*core.BossContext)
		bossCtx.RequestCtx = fctx
		bossCtx.User = nil
		// Очищаем Store, но переиспользуем ту же мапу
		for k := range bossCtx.Store {
			delete(bossCtx.Store, k)
		}
		maps.Copy(bossCtx.Store, app.dependencies)

		var err error
		defer func() {
			if r := recover(); r != nil {
				log.Printf("Panic: %v\n%s", r, debug.Stack())
				err = fmt.Errorf("panic: %v", r)
			}
			// Возвращаем в пул
			bossCtx.RequestCtx = nil
			bossCtx.User = nil
			// store очистится при следующем Get (новая мапа)
			bossCtxPool.Put(bossCtx)
		}()

		err = chain(bossCtx)
		if err != nil {
			fctx.Error("Internal Server Error", fasthttp.StatusInternalServerError)
		}
	}
}

// Static регистрирует маршрут для отдачи статических файлов из указанной директории.
// Например: app.Static("/static", "./public")
func (app *BossApp) Static(prefix, dir string) {
	fs := &fasthttp.FS{
		Root:               dir,
		IndexNames:         []string{"index.html"},
		GenerateIndexPages: true,
		Compress:           true, // автоматически сжимает CSS/JS/html
		AcceptByteRange:    true,
	}
	handler := fs.NewRequestHandler()
	// Обрезаем слеш в конце prefix, если есть, и добавляем "/*"
	path := prefix + "/{file:*}"
	app.router.GET(path, handler)
}

// StaticFS регистрирует обработчик для раздачи файлов из embed.FS
func (app *BossApp) StaticFS(prefix string, fsys fs.FS) {
    // Создаем объект FS, передавая ему корневую папку и твой fs.FS
    fs := &fasthttp.FS{
        Root:               ".",  // Корень файловой системы
        FS:                 fsys, // Твой embed.FS
        GenerateIndexPages: false,
        AcceptByteRange:    true,
    }
    // Получаем обработчик запросов из структуры FS
    handler := fs.NewRequestHandler()

    // Регистрируем маршрут
    app.router.GET(prefix+"/{file:*}", handler)
}

// Run запускает сервер на указанном адресе с корректным graceful shutdown.
func (app *BossApp) Run(addr string) error {
	srv := &fasthttp.Server{
		Handler:      app.router.Handler,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
	}

	if app.config.Server.EnableGzip {
		srv.Handler = fasthttp.CompressHandler(srv.Handler)
	}

	if app.config.Server.EnableHTTP2 {
		// Настройка TLS. Сертификаты можно сгенерировать для разработки.
		cert, err := tls.LoadX509KeyPair("server.crt", "server.key")
		if err != nil {
			log.Fatalf("Ошибка загрузки TLS-сертификатов: %v", err)
		}

		tlsConfig := &tls.Config{
			Certificates: []tls.Certificate{cert},
			NextProtos:   []string{"h2", "http/1.1"}, // Включаем поддержку HTTP/2
		}

		// Создаем TLS-обертку для нашего адреса
		ln, err := tls.Listen("tcp", addr, tlsConfig)
		if err != nil {
			log.Fatalf("Ошибка запуска TLS-листенера: %v", err)
		}
		ln = tls.NewListener(ln, tlsConfig)

		// Добавляем поддержку HTTP/2 к нашему fasthttp-серверу
		http2.ConfigureServer(srv, http2.ServerConfig{})

		// Запускаем сервер с TLS и HTTP/2
		go func() {
			log.Printf("🚀 Awesome Boss запущен на %s (TLS + HTTP/2)", addr)
			if err := srv.Serve(ln); err != nil {
				log.Printf("Ошибка сервера: %v", err)
			}
		}()
	} else {
		// Обычный запуск без TLS и HTTP/2
		go func() {
			log.Printf("🚀 Awesome Boss запущен на %s", addr)
			if err := srv.ListenAndServe(addr); err != nil {
				log.Printf("Ошибка сервера: %v", err)
			}
		}()
	}

	// Ожидаем сигнал завершения
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	<-quit

	log.Println("🛑 Завершение работы...")

	// Создаём контекст с таймаутом
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Используем ShutdownWithContext — он принимает контекст
	if err := srv.ShutdownWithContext(ctx); err != nil {
		log.Printf("Ошибка при завершении: %v", err)
		return err
	}

	log.Println("✅ Сервер остановлен корректно")
	return nil
}

// Выполнено с любовью для Босса 🐈‍
