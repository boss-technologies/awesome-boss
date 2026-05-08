package core

import (
    "testing"
    "github.com/valyala/fasthttp"
)

// setupTestCtx создаёт минимальный BossContext для тестов.
func setupTestCtx() *BossContext {
    fctx := &fasthttp.RequestCtx{}
    fctx.Request.Header.SetMethod("GET")
    fctx.Request.SetRequestURI("/test")
    return NewBossContext(fctx)
}

func TestBossContext_SetGet(t *testing.T) {
    ctx := setupTestCtx()
    ctx.Set("boss", "Максим")
    val := ctx.Get("boss")
    if val != "Максим" {
        t.Errorf("ожидалось 'Максим', получено %v", val)
    }
}

func TestBossContext_GetTyped(t *testing.T) {
    ctx := setupTestCtx()
    ctx.Set("age", 12)
    age, ok := GetTyped[int](ctx, "age")
    if !ok || age != 12 {
        t.Errorf("GetTyped[int] не удался, ok=%v age=%v", ok, age)
    }

    // Тест на неверный тип
    _, ok = GetTyped[string](ctx, "age")
    if ok {
        t.Error("GetTyped[string] должно вернуть false для int")
    }
}

func TestBossContext_JSON(t *testing.T) {
    ctx := setupTestCtx()
    data := map[string]string{"message": "Привет от Босса!"}
    err := ctx.JSON(fasthttp.StatusOK, data)
    if err != nil {
        t.Fatalf("JSON() ошибка: %v", err)
    }
    if ct := string(ctx.Response.Header.ContentType()); ct != "application/json" {
        t.Errorf("Content-Type должен быть application/json, а не %s", ct)
    }
    if status := ctx.Response.StatusCode(); status != fasthttp.StatusOK {
        t.Errorf("статус ответа должен быть 200, а не %d", status)
    }
}

func TestBossContext_WriteString(t *testing.T) {
    ctx := setupTestCtx()
    ctx.WriteString("Мяу!")
    if string(ctx.Response.Body()) != "Мяу!" {
        t.Error("WriteString не сработал")
    }
}

func TestBossContext_Param(t *testing.T) {
    ctx := setupTestCtx()
    ctx.SetUserValue("boss", "Босс")
    if val := ctx.Param("boss"); val != "Босс" {
        t.Errorf("Param(boss) должно быть 'Босс', получено %s", val)
    }
}

// Выполнено с любовью для Босса 🐈‍