package core

import (
    "math"
)

// Page представляет одну страницу данных с метаинформацией.
type Page[T any] struct {
    Items      []T   `json:"items"`
    Page       int   `json:"page"`
    PageSize   int   `json:"page_size"`
    Total      int   `json:"total"`
    TotalPages int   `json:"total_pages"`
    HasPrev    bool  `json:"has_prev"`
    HasNext    bool  `json:"has_next"`
}

// Paginator хранит параметры пагинации, извлечённые из запроса.
type Paginator struct {
    Page     int
    PageSize int
    Offset   int
}

// Validate проверяет и исправляет значения пагинации.
func (p *Paginator) Validate() {
    if p.Page < 1 {
        p.Page = 1
    }
    if p.PageSize < 1 || p.PageSize > 100 { // limit для безопасности
        p.PageSize = 10
    }
    p.Offset = (p.Page - 1) * p.PageSize
}

// NewPage создаёт Page из слайса и параметров пагинации.
func NewPage[T any](items []T, paginator Paginator, total int) Page[T] {
    totalPages := int(math.Ceil(float64(total) / float64(paginator.PageSize)))
    return Page[T]{
        Items:      items,
        Page:       paginator.Page,
        PageSize:   paginator.PageSize,
        Total:      total,
        TotalPages: totalPages,
        HasPrev:    paginator.Page > 1,
        HasNext:    paginator.Page < totalPages,
    }
}