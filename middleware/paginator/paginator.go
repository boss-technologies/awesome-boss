package middleware

import (
    "strconv"

    "github.com/boss-technologies/awesome-boss/core"
)

const (
    DefaultPageSize = 10
    MaxPageSize     = 100
)

func Paginate(next core.Handler) core.Handler {
    return func(ctx *core.BossContext) error {
        p := core.Paginator{
            Page:     1,
            PageSize: DefaultPageSize,
        }

        if pageStr := string(ctx.QueryArgs().Peek("page")); pageStr != "" {
            if page, err := strconv.Atoi(pageStr); err == nil {
                p.Page = page
            }
        }
        if sizeStr := string(ctx.QueryArgs().Peek("page_size")); sizeStr != "" {
            if size, err := strconv.Atoi(sizeStr); err == nil && size <= MaxPageSize {
                p.PageSize = size
            }
        }

        p.Validate()
        ctx.Set("paginator", p)
        return next(ctx)
    }
}