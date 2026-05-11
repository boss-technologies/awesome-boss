package fields

import (
	"database/sql/driver"
	"fmt"
	"regexp"
	"strings"
)

// SlugField хранит «слаг» — URL‑безопасную версию строки.
// При создании через NewSlug авто‑матически генерирует слаг из исходной строки.
type SlugField struct {
	Val string
	Valid bool
}

// Scan реализует pgx.Scanner.
func (s *SlugField) Scan(src any) error {
	if src == nil {
		s.Val = ""
		s.Valid = false
		return nil
	}
	var str string
	switch v := src.(type) {
	case string:
		str = v
	case []byte:
		str = string(v)
	default:
		return fmt.Errorf("slug: cannot scan %T into SlugField", src)
	}
	s.Val = str
	s.Valid = true
	return nil
}

// Value реализует driver.Valuer.
func (s SlugField) Value() (driver.Value, error) {
	if !s.Valid {
		return nil, nil
	}
	return s.Value, nil
}

// NewSlug создаёт SlugField из строки, генерируя слаг.
func NewSlug(input string) SlugField {
	return SlugField{
		Val: Slugify(input),
		Valid: true,
	}
}

// Slugify преобразует строку в URL‑безопасный слаг.
// Пример: "Привет, мир!" -> "privet-mir"
func Slugify(input string) string {
	// Приводим к нижнему регистру
	s := strings.ToLower(input)

	// Заменяем пробелы и подчёркивания на дефисы
	s = strings.ReplaceAll(s, " ", "-")
	s = strings.ReplaceAll(s, "_", "-")

	// Удаляем все символы, кроме букв, цифр, дефиса
	reg := regexp.MustCompile(`[^a-z0-9\-]+`)
	s = reg.ReplaceAllString(s, "")

	// Убираем повторяющиеся дефисы
	multipleDashes := regexp.MustCompile(`-+`)
	s = multipleDashes.ReplaceAllString(s, "-")

	// Обрезаем дефисы по краям
	s = strings.Trim(s, "-")

	return s
}

// String возвращает слаг как строку.
func (s SlugField) String() string {
	if !s.Valid {
		return ""
	}
	return s.Val
}

// Выполнено с любовью для Босса 🐈