// Package fields содержит кастомные типы полей для BossQ.
package fields

import (
	"database/sql/driver"
	"fmt"
)

// Choices — обобщённое поле, которое хранит значение перечислимого типа T.
// T должен быть основан на строковом типе (например, type Status string).
//
// Пример использования:
//
//	type Status string
//	const (
//	    StatusDraft     Status = "draft"
//	    StatusPublished Status = "published"
//	)
//
//	type MyModel struct {
//	    Status Choices[Status] `bossq:"status"`
//	}
type Choices[T ~string] struct {
	Val   T   
	Valid bool // true, если значение было прочитано из БД и не nil
}

// Scan реализует pgx.Scanner.
func (c *Choices[T]) Scan(src any) error {
	if src == nil {
		c.Val = T("")
		c.Valid = false
		return nil
	}
	var s string
	switch v := src.(type) {
	case string:
		s = v
	case []byte:
		s = string(v)
	default:
		return fmt.Errorf("choices: cannot scan %T into Choices", src)
	}
	c.Val = T(s)
	c.Valid = true
	return nil
}

// Value реализует driver.Valuer.
func (c Choices[T]) Value() (driver.Value, error) {
	if !c.Valid {
		return nil, nil
	}
	return string(c.Val), nil
}

// NewChoice создаёт Choices с явно заданным значением.
func NewChoice[T ~string](val T) Choices[T] {
	return Choices[T]{Val: val, Valid: true}
}

// IsValid проверяет, входит ли значение в список разрешённых.
// Удобно использовать в методах валидации модели.
func (c Choices[T]) IsValid(allowed ...T) bool {
	if !c.Valid {
		return false
	}
	for _, a := range allowed {
		if c.Val == a {
			return true
		}
	}
	return false
}

// String возвращает строковое представление.
func (c Choices[T]) String() string {
	if !c.Valid {
		return ""
	}
	return string(c.Val)
}

// Выполнено с любовью для Босса 🐈