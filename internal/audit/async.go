package audit

import (
	"sync"
)

type AsyncWriter struct {
	writer   Writer
	queue    chan Entry[string]
	fallback *FallbackLogger // резервный писатель при переполнении
	wg       sync.WaitGroup
	closeCh  chan struct{}
}

func NewAsyncWriter(w Writer, bufferSize int, workers int, fallback *FallbackLogger) *AsyncWriter {
	aw := &AsyncWriter{
		writer:   w,
		queue:    make(chan Entry[string], bufferSize),
		fallback: fallback,
		closeCh:  make(chan struct{}),
	}
	for range workers {
		aw.wg.Add(1)
		go aw.worker()
	}
	return aw
}

func (aw *AsyncWriter) worker() {
	defer aw.wg.Done()
	for entry := range aw.queue {
		_ = aw.writer.Write(entry)
	}
}

func (aw *AsyncWriter) Write(entry Entry[string]) {
	select {
	case aw.queue <- entry:
	default:
		// При переполнении основного канала пишем в fallback (синхронно)
		if aw.fallback != nil {
			aw.fallback.Log(entry)
		}
	}
}

func (aw *AsyncWriter) Close() error {
	close(aw.queue)   // сигнал, что новых записей не будет
	aw.wg.Wait()      // ожидание завершения всех воркеров
	close(aw.closeCh)
	return aw.writer.Close()
}

// Выполнено с любовью для Босса 🐈‍