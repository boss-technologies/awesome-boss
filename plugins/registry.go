package plugin

import (
	"fmt"
	"sync"

	"github.com/boss-technologies/awesome-boss/core"
)

// Registry — это центральное хранилище всех активных плагинов приложения.
type Registry struct {
    plugins     map[string]BossPlugin
    requestHooks []RequestHook
    mu          sync.RWMutex // Потокобезопасность
}

// Глобальная переменная реестра.
var GlobalRegistry = &Registry{
    plugins:     make(map[string]BossPlugin),
    requestHooks: make([]RequestHook, 0),
}

// Register добавляет плагин в глобальный реестр.
func Register(p BossPlugin) error {
    GlobalRegistry.mu.Lock()
    defer GlobalRegistry.mu.Unlock()

    if _, exists := GlobalRegistry.plugins[p.Name()]; exists {
        return fmt.Errorf("плагин '%s' уже зарегистрирован", p.Name())
    }
    GlobalRegistry.plugins[p.Name()] = p

    // Если плагин реализует интерфейс RequestHook, сохраняем его в отдельный список.
    if hook, ok := p.(RequestHook); ok {
        GlobalRegistry.requestHooks = append(GlobalRegistry.requestHooks, hook)
    }

    return nil
}

// ApplyRequestHooksBefore применяет все BeforeRequest хуки.
func (r *Registry) ApplyRequestHooksBefore(ctx *core.BossContext) error {
    r.mu.RLock()
    defer r.mu.RUnlock()
    for _, hook := range r.requestHooks {
        if err := hook.BeforeRequest(ctx); err != nil {
            return err
        }
    }
    return nil
}

// Выполнено с любовью для Босса 🐈‍