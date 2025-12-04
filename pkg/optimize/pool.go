package optimize

import (
	"sync"
)

// BytePool is a pool of byte slices to reduce allocations
type BytePool struct {
	pool sync.Pool
	size int
}

// NewBytePool creates a new byte pool with specified size
func NewBytePool(size int) *BytePool {
	return &BytePool{
		size: size,
		pool: sync.Pool{
			New: func() interface{} {
				return make([]byte, size)
			},
		},
	}
}

// Get gets a byte slice from the pool
func (p *BytePool) Get() []byte {
	return p.pool.Get().([]byte)
}

// Put returns a byte slice to the pool
func (p *BytePool) Put(b []byte) {
	// Only put back if it's the right size
	if cap(b) >= p.size {
		p.pool.Put(b[:p.size])
	}
}

// StringPool is a pool of strings for frequently used strings
type StringPool struct {
	pool sync.Pool
}

// NewStringPool creates a new string pool
func NewStringPool() *StringPool {
	return &StringPool{
		pool: sync.Pool{
			New: func() interface{} {
				return make(map[string]string, 16)
			},
		},
	}
}

// Get gets a string map from the pool
func (p *StringPool) Get() map[string]string {
	return p.pool.Get().(map[string]string)
}

// Put returns a string map to the pool (clears it first)
func (p *StringPool) Put(m map[string]string) {
	// Clear map before returning
	for k := range m {
		delete(m, k)
	}
	p.pool.Put(m)
}

