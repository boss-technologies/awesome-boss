package bosssocket

import "sync"

// FixedBufferPool — кастомный пул буферов с ограничением максимального размера.
type FixedBufferPool struct {
	pool    sync.Pool
	maxSize int
}

func NewFixedBufferPool(bufSize, maxSize int) *FixedBufferPool {
	return &FixedBufferPool{
		maxSize: maxSize,
		pool: sync.Pool{
			New: func() any {
				buf := make([]byte, bufSize)
				return &buf
			},
		},
	}
}

func (p *FixedBufferPool) Get() []byte {
	bufPtr := p.pool.Get().(*[]byte)
	return (*bufPtr)[:cap(*bufPtr)]
}

func (p *FixedBufferPool) Put(buf []byte) {
	if cap(buf) <= p.maxSize {
		p.pool.Put(&buf)
	}
	// Буферы больше maxSize не возвращаются в пул, освобождая память
}

// Выполнено с любовью для Босса 🐈‍
