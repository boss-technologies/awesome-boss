# Awesome Boss – FinTech-фреймворк на Go, вдохновлённый котом Боссом! 🐱

[![License: AGPL v3](https://img.shields.io/badge/License-AGPL%20v3-blue.svg)](https://www.gnu.org/licenses/agpl-3.0)
[![Go version](https://img.shields.io/badge/Go-1.26+-blue.svg)](https://go.dev/)

**Awesome Boss** — это лёгкий, быстрый и безопасный фреймворк для Go.  
Он сочетает производительность `fasthttp` с простотой **MTH-архитектуры** (Model–Templates–Handler).

> 🧪 **Текущая версия: 0.2 ** – ядро готово, BAT, Boss Socket, `boss.toml` готовы - следующие версии добавят ещё больше**

---

## ✨ Быстрый старт

```bash
go install github.com/boss-technologies/awesome-boss/cmd/boss@latest
boss new myapp --template normal
cd myapp
boss run
```

Готово! Сервер запущен на `http://localhost:8080`.

### Пример кода

```go
package main

import (
    "log"
    "github.com/boss-technologies/awesome-boss"
    "github.com/boss-technologies/awesome-boss/config"
    "github.com/boss-technologies/awesome-boss/core"
)

func main() {
    cfg := &config.BossConfig{Mode: config.ModeNormal, Addr: ":8080"}
    app := awesomeboss.New(cfg)

    app.Get("/", func(ctx *core.BossContext) error {
        return ctx.JSON(200, map[string]string{"message": "Привет от кота Босса! 🐈"})
    })

    log.Fatal(app.Run(cfg.Addr))
}
```

---

## 📦 Особенности

- 🚀 **Молниеносная скорость** – `fasthttp` + Go 1.26
- 🧩 **MTH-архитектура** – Model, Templates, Handler. Никакой магии.
- 🪄 **Middleware** – глобальные и на уровне групп
- 🧶 **Шаблонизатор Fur** – на основе `html/template`, безопасный от XSS
- 🔐 **BAT** (Boss Auth Token) – токены на PASETO
- 📡 **Boss Socket** – высокопроизводительный WebSocket 
- 🏦 **Два режима**: `normal` (быстрый старт) и `fintech` (Decimal, аудит, DDoS-защита)
- 🧩 **Плагины** – будут в 0.5

---

## 🗺️ Дорожная карта (Roadmap)

| Версия | Что появится |
|--------|--------------|
| **0.1** | ✅ Роутер, Fur, CLI, два режима |
| **0.2** | ✅ BAT, Boss Socket, `boss.toml` |
| **0.3** | 🔜 Аудит, безопасность, DDoS-защита |
| **0.4** | BossQ (ORM + Decimal) |
| **0.5** | Система плагинов, улучшение WebSocket |
| **0.6** | Батарейки - куда-же без них! |
| **0.7** | Батареек много не бывает 🧰 |
| **0.8** | Boss Socket (рост производительности) |
| **0.9** | Boss Admin (админ-панель), генерация моделий (boss generate model), кеширование |
| **1.0** | Юбилей, полноценная работа с плагинами |

> 👉 **Сейчас версия 0.2**: - ядро готово, BAT, Boss Socket, `boss.toml` готовы - следующие версии добавят ещё больше 

---

## ⚙️ Установка CLI

```bash
go install github.com/boss-technologies/awesome-boss/cmd/boss@latest
```

Проверь: `boss version`

## 📄 Лицензия

AGPL v3 – подробности в файле [LICENSE](LICENSE).

---

## 🐈 Благодарности

Вдохновлено моим котом Боссом, который нажимает лапой на клавиатуру, когда я пишу код.  
Сделано с любовью