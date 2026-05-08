package audit

import "time"

type Entry[T any] struct {
	Timestamp   time.Time `json:"timestamp"`
	RequestID   string    `json:"request_id"`
	Method      string    `json:"method"`
	Path        string    `json:"path"`
	StatusCode  int       `json:"status_code"`
	Duration    int64     `json:"duration_ms"`
	UserID      T         `json:"user_id"` // из BAT
	RemoteIP    string    `json:"remote_ip"`
	UserAgent   string    `json:"user_agent"`
	RequestBody string    `json:"request_body,omitempty"` // опционально
	Response    any       `json:"response,omitempty"`     // если нужно
	Error       string    `json:"error,omitempty"`
}

// Выполнено с любовью для Босса 🐈‍
