package audit

import (
    "encoding/json"
    "os"
    "sync"
)

// FallbackLogger синхронно пишет записи в файл при переполнении основного канала.
type FallbackLogger struct {
    mu   sync.Mutex // защищает доступ к файлу
    file *os.File
}

func NewFallbackLogger(path string) (*FallbackLogger, error) {
    f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
    if err != nil {
        return nil, err
    }
    return &FallbackLogger{file: f}, nil
}

func (fl *FallbackLogger) Log(entry Entry[string]) {
    fl.mu.Lock()
    defer fl.mu.Unlock()
    // Сериализуем в JSON и дописываем строкой
    data, err := json.Marshal(entry)
    if err != nil {
        // на крайний случай можно записать "сырой" текст
        fl.file.WriteString("fallback marshal error\n")
        return
    }
    data = append(data, '\n')
    fl.file.Write(data)
}

func (fl *FallbackLogger) Close() error {
    return fl.file.Close()
}

// Выполнено с любовью для Босса 🐈‍