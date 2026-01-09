package memory

import (
	"testing"

	"github.com/discochess/stockpile/internal/store/cachedstore/cachestrategy/lru"
)

func TestBackend_GetSet(t *testing.T) {
	strategy, err := lru.New(10)
	if err != nil {
		t.Fatalf("lru.New() error = %v", err)
	}
	b := New(strategy, nil)

	// Initially empty.
	if _, ok := b.Get(1); ok {
		t.Error("Get() should return false for missing key")
	}

	// Set and get.
	b.Set(1, []byte("hello"))
	data, ok := b.Get(1)
	if !ok {
		t.Error("Get() should return true after Set")
	}
	if string(data) != "hello" {
		t.Errorf("Get() = %q, want %q", data, "hello")
	}
}

func TestBackend_Stats(t *testing.T) {
	strategy, err := lru.New(10)
	if err != nil {
		t.Fatalf("lru.New() error = %v", err)
	}
	b := New(strategy, nil)

	b.Set(1, []byte("data"))

	// Hit.
	b.Get(1)
	// Miss.
	b.Get(2)

	stats := b.Stats()
	if stats.Hits != 1 {
		t.Errorf("Stats().Hits = %d, want 1", stats.Hits)
	}
	if stats.Misses != 1 {
		t.Errorf("Stats().Misses = %d, want 1", stats.Misses)
	}
	if stats.Size != 1 {
		t.Errorf("Stats().Size = %d, want 1", stats.Size)
	}
}

func TestBackend_LRUEviction(t *testing.T) {
	strategy, err := lru.New(2) // Capacity of 2.
	if err != nil {
		t.Fatalf("lru.New() error = %v", err)
	}
	b := New(strategy, nil)

	b.Set(1, []byte("one"))
	b.Set(2, []byte("two"))
	b.Set(3, []byte("three")) // Should evict 1.

	if _, ok := b.Get(1); ok {
		t.Error("Get(1) should return false after eviction")
	}
	if _, ok := b.Get(2); !ok {
		t.Error("Get(2) should return true")
	}
	if _, ok := b.Get(3); !ok {
		t.Error("Get(3) should return true")
	}
}

func TestLRU_InvalidCapacity(t *testing.T) {
	_, err := lru.New(0)
	if err == nil {
		t.Error("lru.New(0) should return error")
	}

	_, err = lru.New(-1)
	if err == nil {
		t.Error("lru.New(-1) should return error")
	}
}

// fakeStrategy is a simple strategy for testing injection.
type fakeStrategy struct {
	data map[int][]byte
}

func (s *fakeStrategy) Get(key int) ([]byte, bool) {
	v, ok := s.data[key]
	return v, ok
}

func (s *fakeStrategy) Add(key int, value []byte) bool {
	s.data[key] = value
	return true
}

func (s *fakeStrategy) Len() int {
	return len(s.data)
}

func TestBackend_InjectableStrategy(t *testing.T) {
	strategy := &fakeStrategy{data: make(map[int][]byte)}
	b := New(strategy, nil)

	b.Set(1, []byte("test"))
	data, ok := b.Get(1)
	if !ok || string(data) != "test" {
		t.Error("injectable strategy should work")
	}
}
