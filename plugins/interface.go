package plugin

import (
	"github.com/boss-technologies/awesome-boss"
	"github.com/boss-technologies/awesome-boss/core"
)

// BossPlugin — это интерфейс, который должен реализовать КАЖДЫЙ плагин.
type BossPlugin interface {
    // Name возвращает уникальное имя плагина (например, "cors").
    Name() string
    // Init вызывается при старте приложения для настройки плагина.
    Init(app *awesomeboss.BossApp) error
	// Version возвращает версию плагина.
	Version() string
}

// RequestHook — пример хука, который позволяет плагинам
// вмешиваться в обработку запроса на разных этапах.
type RequestHook interface {
    BossPlugin // Каждый такой хук — тоже плагин
    // BeforeRequest срабатывает перед вызовом основного обработчика.
    BeforeRequest(ctx *core.BossContext) error
    // AfterRequest срабатывает после вызова основного обработчика.
    AfterRequest(ctx *core.BossContext, handlerErr error)
}

// Выполнено с любовью для Босса 🐈‍