package audit

type Writer interface {
    Write(entry Entry[string]) error
    Close() error
}

// Выполнено с любовью для Босса 🐈‍