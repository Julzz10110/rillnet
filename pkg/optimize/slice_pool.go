package optimize

import (
	"sync"
)

// SlicePool is a pool for reusing slices to reduce allocations
type SlicePool struct {
	pool sync.Pool
	size int
}

// NewSlicePool creates a new slice pool
func NewSlicePool(size int) *SlicePool {
	return &SlicePool{
		size: size,
		pool: sync.Pool{
			New: func() interface{} {
				return make([]interface{}, 0, size)
			},
		},
	}
}

// Get gets a slice from the pool
func (p *SlicePool) Get() []interface{} {
	return p.pool.Get().([]interface{})
}

// Put returns a slice to the pool (clears it first)
func (p *SlicePool) Put(s []interface{}) {
	// Only put back if capacity is reasonable
	if cap(s) <= p.size*2 {
		s = s[:0]
		p.pool.Put(s)
	}
}

// StringSlicePool is a pool for string slices
type StringSlicePool struct {
	pool sync.Pool
	size int
}

// NewStringSlicePool creates a new string slice pool
func NewStringSlicePool(size int) *StringSlicePool {
	return &StringSlicePool{
		size: size,
		pool: sync.Pool{
			New: func() interface{} {
				return make([]string, 0, size)
			},
		},
	}
}

// Get gets a string slice from the pool
func (p *StringSlicePool) Get() []string {
	return p.pool.Get().([]string)
}

// Put returns a string slice to the pool
func (p *StringSlicePool) Put(s []string) {
	if cap(s) <= p.size*2 {
		s = s[:0]
		p.pool.Put(s)
	}
}

// PreAllocateSlice pre-allocates a slice with known capacity
func PreAllocateSlice[T any](length, capacity int) []T {
	if capacity < length {
		capacity = length
	}
	return make([]T, length, capacity)
}

// GrowSlice grows a slice efficiently
func GrowSlice[T any](s []T, newLen int) []T {
	if newLen <= cap(s) {
		return s[:newLen]
	}
	
	// Double capacity strategy
	newCap := cap(s) * 2
	if newCap < newLen {
		newCap = newLen
	}
	
	newSlice := make([]T, newLen, newCap)
	copy(newSlice, s)
	return newSlice
}

