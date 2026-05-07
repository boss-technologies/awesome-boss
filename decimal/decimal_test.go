package decimal

import (
    "encoding/json"
    "testing"
)

func TestDecimal_UnmarshalJSON(t *testing.T) {
    tests := []struct {
        name     string
        input    string
        expected string
        wantErr  bool
    }{
        {"string", `"123.45"`, "123.45", false},
        {"int", `123`, "123", false},
        {"float", `123.45`, "123.45", false},
        {"negative string", `"-100.5"`, "-100.5", false},
        {"invalid string", `"abc"`, "", true},
        {"empty string", `""`, "", true},
        {"null", `null`, "", true}, // null должен возвращать ошибку, если не хотим nil
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            var d Decimal
            err := json.Unmarshal([]byte(tt.input), &d)
            if (err != nil) != tt.wantErr {
                t.Errorf("UnmarshalJSON() error = %v, wantErr %v", err, tt.wantErr)
                return
            }
            if !tt.wantErr && d.String() != tt.expected {
                t.Errorf("got %v, want %v", d.String(), tt.expected)
            }
        })
    }
}

// Выполнено с любовью для Босса 🐈‍