package optimize

import (
	"testing"
)

func BenchmarkBytePool(b *testing.B) {
	pool := NewBytePool(1024)
	b.ResetTimer()
	
	for i := 0; i < b.N; i++ {
		buf := pool.Get()
		// Simulate usage
		buf[0] = byte(i)
		pool.Put(buf)
	}
}

func BenchmarkByteAllocation(b *testing.B) {
	for i := 0; i < b.N; i++ {
		buf := make([]byte, 1024)
		// Simulate usage
		buf[0] = byte(i)
	}
}

func BenchmarkPreAllocateSlice(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s := PreAllocateSlice[int](10, 20)
		_ = s
	}
}

func BenchmarkRegularSlice(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s := make([]int, 10, 20)
		_ = s
	}
}

func BenchmarkGrowSlice(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s := make([]int, 2, 4)
		s = GrowSlice(s, 100)
		_ = s
	}
}

func BenchmarkRegularGrow(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s := make([]int, 2, 4)
		for len(s) < 100 {
			s = append(s, 0)
		}
		_ = s
	}
}

