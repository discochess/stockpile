package cachedstore

import (
	"context"
	"errors"
	"testing"

	"github.com/discochess/stockpile/internal/store"
)

// fakeBackend is a simple in-memory backend for testing.
type fakeBackend struct {
	data   map[int][]byte
	hits   int64
	misses int64
}

func newFakeBackend() *fakeBackend {
	return &fakeBackend{data: make(map[int][]byte)}
}

func (b *fakeBackend) Get(shardID int) ([]byte, bool) {
	if data, ok := b.data[shardID]; ok {
		b.hits++
		return data, true
	}
	b.misses++
	return nil, false
}

func (b *fakeBackend) Set(shardID int, data []byte) {
	b.data[shardID] = data
}

func (b *fakeBackend) Stats() Stats {
	return Stats{Hits: b.hits, Misses: b.misses, Size: len(b.data)}
}

// fakeStore is a simple store for testing.
type fakeStore struct {
	data map[int][]byte
}

func newFakeStore() *fakeStore {
	return &fakeStore{data: make(map[int][]byte)}
}

func (s *fakeStore) ReadShard(ctx context.Context, shardID int) ([]byte, error) {
	if data, ok := s.data[shardID]; ok {
		return data, nil
	}
	return nil, store.ErrNotFound
}

func (s *fakeStore) Close() error {
	return nil
}

func TestStore_CacheHit(t *testing.T) {
	backend := newFakeBackend()
	underlying := newFakeStore()

	// Pre-populate cache.
	backend.Set(1, []byte("cached data"))

	s := New(underlying, backend)
	ctx := context.Background()

	data, err := s.ReadShard(ctx, 1)
	if err != nil {
		t.Fatalf("ReadShard() error = %v", err)
	}

	if string(data) != "cached data" {
		t.Errorf("ReadShard() = %q, want %q", data, "cached data")
	}

	stats := s.Stats()
	if stats.Hits != 1 {
		t.Errorf("Stats().Hits = %d, want 1", stats.Hits)
	}
}

func TestStore_CacheMiss(t *testing.T) {
	backend := newFakeBackend()
	underlying := newFakeStore()

	// Put data in underlying store, not cache.
	underlying.data[1] = []byte("underlying data")

	s := New(underlying, backend)
	ctx := context.Background()

	data, err := s.ReadShard(ctx, 1)
	if err != nil {
		t.Fatalf("ReadShard() error = %v", err)
	}

	if string(data) != "underlying data" {
		t.Errorf("ReadShard() = %q, want %q", data, "underlying data")
	}

	// Should have cached the data.
	if _, ok := backend.data[1]; !ok {
		t.Error("data should be cached after miss")
	}

	stats := s.Stats()
	if stats.Misses != 1 {
		t.Errorf("Stats().Misses = %d, want 1", stats.Misses)
	}
}

func TestStore_NotFound(t *testing.T) {
	backend := newFakeBackend()
	underlying := newFakeStore()

	s := New(underlying, backend)
	ctx := context.Background()

	_, err := s.ReadShard(ctx, 999)
	if !errors.Is(err, store.ErrNotFound) {
		t.Errorf("ReadShard() error = %v, want ErrNotFound", err)
	}
}

func TestStats_HitRate(t *testing.T) {
	tests := []struct {
		name     string
		hits     int64
		misses   int64
		expected float64
	}{
		{"no requests", 0, 0, 0},
		{"all hits", 10, 0, 100},
		{"all misses", 0, 10, 0},
		{"50% hit rate", 5, 5, 50},
		{"75% hit rate", 3, 1, 75},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := Stats{Hits: tt.hits, Misses: tt.misses}
			if got := s.HitRate(); got != tt.expected {
				t.Errorf("HitRate() = %v, want %v", got, tt.expected)
			}
		})
	}
}
