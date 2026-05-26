package decimal

import (
	"encoding/json"
	"fmt"

	"github.com/shopspring/decimal"
	"database/sql/driver"
)

// Decimal — неизменяемый тип для точных денежных вычислений.
// Это надстройка над shopspring/decimal с поддержкой JSON и pgx.
type Decimal struct {
	d decimal.Decimal
}

// NewDecimal — основной конструктор. Гарантирует, что значение всегда установлено.
func NewDecimal(value decimal.Decimal) Decimal {
	return Decimal{d: value}
}

// Scan реализует интерфейс pgx.Scanner
func (d *Decimal) Scan(src any) error {
    if src == nil {
        d.d = decimal.Zero
        return nil
    }
    var s string
    switch v := src.(type) {
    case string:
        s = v
    case []byte:
        s = string(v)
    default:
        return fmt.Errorf("decimal: cannot scan %T into Decimal", src)
    }
    dec, err := decimal.NewFromString(s)
    if err != nil {
        return err
    }
    d.d = dec
    return nil
}

// Value реализует driver.Valuer
func (d Decimal) Value() (driver.Value, error) {
    return d.d.String(), nil
}

// String возвращает строковое представление.
func (d Decimal) String() string {
	return d.d.String()
}

// StringFixed возвращает строку с фиксированным числом знаков после запятой.
func (d Decimal) StringFixed(places int32) string {
	return d.d.StringFixed(places)
}

// ---------------------------------------------------------------------------
// JSON
// ---------------------------------------------------------------------------

// MarshalJSON сериализует Decimal как строку с двумя знаками после запятой.
func (d Decimal) MarshalJSON() ([]byte, error) {
	s := d.d.StringFixed(2)
	return json.Marshal(s)
}

// UnmarshalJSON десериализует Decimal из строки.
func (d *Decimal) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return fmt.Errorf("decimal: money values must be strings, got %s", string(data))
	}
	dec, err := decimal.NewFromString(s)
	if err != nil {
		return fmt.Errorf("decimal: invalid string value %q: %w", s, err)
	}
	d.d = dec
	return nil
}

// ---------------------------------------------------------------------------
// Арифметика (неизменяемая, возвращает новый Decimal)
// ---------------------------------------------------------------------------

func (d Decimal) Add(other Decimal) Decimal {
	return Decimal{d: d.d.Add(other.d)}
}

func (d Decimal) Sub(other Decimal) Decimal {
	return Decimal{d: d.d.Sub(other.d)}
}

func (d Decimal) Mul(other Decimal) Decimal {
	return Decimal{d: d.d.Mul(other.d)}
}

func (d Decimal) Div(other Decimal) Decimal {
	return Decimal{d: d.d.Div(other.d)}
}

// ---------------------------------------------------------------------------
// Сравнение
// ---------------------------------------------------------------------------

func (d Decimal) Equal(other Decimal) bool {
	return d.d.Equal(other.d)
}

func (d Decimal) GreaterThan(other Decimal) bool {
	return d.d.GreaterThan(other.d)
}

func (d Decimal) LessThan(other Decimal) bool {
	return d.d.LessThan(other.d)
}

func (d Decimal) IsZero() bool {
	return d.d.Equal(decimal.Zero)
}

func (d Decimal) IsNegative() bool {
	return d.d.LessThan(decimal.Zero)
}

// ---------------------------------------------------------------------------
// Округление (неизменяемое, возвращает новый Decimal)
// ---------------------------------------------------------------------------

func (d Decimal) Round(places int32) Decimal {
	return Decimal{d: d.d.Round(places)}
}

// ---------------------------------------------------------------------------
// Удобные конструкторы
// ---------------------------------------------------------------------------

// FromFloat64 создаёт Decimal из float64.
func FromFloat64(val float64) Decimal {
	return Decimal{d: decimal.NewFromFloat(val)}
}

// FromString создаёт Decimal из строки.
func FromString(s string) (Decimal, error) {
	d, err := decimal.NewFromString(s)
	if err != nil {
		return Decimal{}, fmt.Errorf("decimal: неверная строка %q: %w", s, err)
	}
	return Decimal{d: d}, nil
}
// Выполнено с любовью для Босса 🐈‍