// Package lru implements an LRU cache eviction strategy.
package lru

import (
	"github.com/discochess/stockpile/internal/store/cachedstore/cachestrategy"
	lru "github.com/hashicorp/golang-lru/v2"
)

// Compile-time check that Strategy implements cachestrategy.Strategy.
var _ cachestrategy.Strategy = (*Strategy)(nil)

// Strategy implements LRU eviction.
type Strategy struct {
	cache *lru.Cache[int, []byte]
}

// New creates a new LRU strategy with the given capacity.
func New(capacity int) (*Strategy, error) {
	c, err := lru.New[int, []byte](capacity)
	if err != nil {
		return nil, err
	}
	return &Strategy{cache: c}, nil
}

// Get retrieves a value by key.
func (s *Strategy) Get(key int) ([]byte, bool) {
	return s.cache.Get(key)
}

// Add adds a value to the cache.
func (s *Strategy) Add(key int, value []byte) bool {
	return s.cache.Add(key, value)
}

// Len returns the number of items in the cache.
func (s *Strategy) Len() int {
	return s.cache.Len()
}
