package awesomeboss

import (
	"bytes"
	"html/template"
	"log"
	"sync"

	"github.com/boss-technologies/awesome-boss/core"

	"github.com/valyala/fasthttp"
)

// Fur - это простой шаблонизатор для Awesome Boss, вдохновленный кошачьей шерстью 🐾
// Название "Fur" (шерсть) выбрано в честь моего кота Босса.

// bufPool переиспользует bytes.Buffer, чтобы не нагружать сборщик мусора.
var bufPool = sync.Pool{
	New: func() any {
		return new(bytes.Buffer)
	},
}

type Fur struct {
	templates *template.Template
}

// NewFur создает новый экземпляр шаблонизатора Fur
func NewFur(patterns string) (*Fur, error) {
	tmpl, err := template.ParseGlob(patterns)
	if err != nil {
		return nil, err
	}
	return &Fur{templates: tmpl}, nil
}

// Render выполняет рендеринг шаблона с данными, используя пул буферов для скорости.
func (f *Fur) Render(ctx *core.BossContext, templateName string, data any) error {
	// Берем чистый буфер из пула
	buf := bufPool.Get().(*bytes.Buffer)
	buf.Reset()            // на всякий случай (вдруг кто-то забыл)
	defer bufPool.Put(buf) // возвращаем в пул по завершении

	if err := f.templates.ExecuteTemplate(buf, templateName, data); err != nil {
		log.Printf("Ошибка рендеринга шаблона %s: %v", templateName, err)
		ctx.Response.SetStatusCode(fasthttp.StatusInternalServerError)
		ctx.Response.SetBodyString("Internal Server Error")
		return err
	}

	ctx.Response.Header.SetContentType("text/html; charset=utf-8")
	ctx.Response.SetBody(buf.Bytes())
	return nil
}

// RenderPartial рендерит только указанный шаблон БЕЗ основного макета.
// Идеально для HTMX-запросов.
func (f *Fur) RenderPartial(ctx *core.BossContext, templateName string, data any) error {
    buf := bufPool.Get().(*bytes.Buffer)
    buf.Reset()
    defer bufPool.Put(buf)

    // Выполняем только указанный шаблон, а не весь набор с макетом
    if err := f.templates.ExecuteTemplate(buf, templateName, data); err != nil {
        log.Printf("Ошибка рендеринга partial %s: %v", templateName, err)
        ctx.Response.SetStatusCode(fasthttp.StatusInternalServerError)
        ctx.Response.SetBodyString("Internal Server Error")
        return err
    }

    ctx.Response.Header.SetContentType("text/html; charset=utf-8")
    ctx.Response.SetBody(buf.Bytes())
    return nil
}

// Выполнено с любовью для Босса 🐈
