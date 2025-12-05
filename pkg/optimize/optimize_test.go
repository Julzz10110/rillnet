package optimize

import (
	"testing"
)

func TestBytePool(t *testing.T) {
	pool := NewBytePool(1024)
	
	// Get buffer
	buf := pool.Get()
	if len(buf) != 1024 {
		t.Errorf("expected buffer size 1024, got %d", len(buf))
	}
	
	// Put back
	pool.Put(buf)
	
	// Get again (should reuse)
	buf2 := pool.Get()
	if len(buf2) != 1024 {
		t.Errorf("expected buffer size 1024, got %d", len(buf2))
	}
}

func TestStringPool(t *testing.T) {
	pool := NewStringPool()
	
	// Get map
	m := pool.Get()
	if m == nil {
		t.Error("expected non-nil map")
	}
	
	// Use map
	m["key"] = "value"
	
	// Put back
	pool.Put(m)
	
	// Get again (should be cleared)
	m2 := pool.Get()
	if len(m2) != 0 {
		t.Errorf("expected empty map, got %d keys", len(m2))
	}
}

func TestPreAllocateSlice(t *testing.T) {
	// Test with length and capacity
	s := PreAllocateSlice[int](5, 10)
	if len(s) != 5 {
		t.Errorf("expected length 5, got %d", len(s))
	}
	if cap(s) != 10 {
		t.Errorf("expected capacity 10, got %d", cap(s))
	}
	
	// Test with capacity less than length
	s2 := PreAllocateSlice[int](10, 5)
	if len(s2) != 10 {
		t.Errorf("expected length 10, got %d", len(s2))
	}
	if cap(s2) < 10 {
		t.Errorf("expected capacity >= 10, got %d", cap(s2))
	}
}

func TestGrowSlice(t *testing.T) {
	// Start with small slice
	s := make([]int, 2, 4)
	s[0] = 1
	s[1] = 2
	
	// Grow to larger size
	s = GrowSlice(s, 10)
	if len(s) != 10 {
		t.Errorf("expected length 10, got %d", len(s))
	}
	if s[0] != 1 || s[1] != 2 {
		t.Error("expected original values to be preserved")
	}
	
	// Grow to same size (should not reallocate)
	oldCap := cap(s)
	s = GrowSlice(s, 10)
	if cap(s) != oldCap {
		t.Error("expected no reallocation for same size")
	}
}

func TestSlicePool(t *testing.T) {
	pool := NewSlicePool(10)
	
	// Get slice
	s := pool.Get()
	if cap(s) != 10 {
		t.Errorf("expected capacity 10, got %d", cap(s))
	}
	
	// Use slice
	s = append(s, 1, 2, 3)
	
	// Put back
	pool.Put(s)
	
	// Get again (should be cleared)
	s2 := pool.Get()
	if len(s2) != 0 {
		t.Errorf("expected empty slice, got length %d", len(s2))
	}
}

func TestStringSlicePool(t *testing.T) {
	pool := NewStringSlicePool(10)
	
	// Get slice
	s := pool.Get()
	if cap(s) != 10 {
		t.Errorf("expected capacity 10, got %d", cap(s))
	}
	
	// Use slice
	s = append(s, "a", "b", "c")
	
	// Put back
	pool.Put(s)
	
	// Get again (should be cleared)
	s2 := pool.Get()
	if len(s2) != 0 {
		t.Errorf("expected empty slice, got length %d", len(s2))
	}
}

