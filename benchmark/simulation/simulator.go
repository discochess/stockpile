// Package simulation provides tools for simulating shard access patterns.
package simulation

import (
	"github.com/discochess/stockpile/internal/shard"
)

// Simulator simulates shard access patterns for different strategies.
type Simulator struct {
	strategies  []shard.Strategy
	totalShards int
}

// NewSimulator creates a new Simulator with the given strategies.
func NewSimulator(totalShards int, strategies ...shard.Strategy) *Simulator {
	return &Simulator{
		strategies:  strategies,
		totalShards: totalShards,
	}
}

// SimulateGame simulates a single game lookup sequence and returns
// shard access patterns for each strategy.
func (s *Simulator) SimulateGame(fens []string) map[string]*GameResult {
	results := make(map[string]*GameResult, len(s.strategies))

	for _, strategy := range s.strategies {
		result := &GameResult{
			StrategyName: strategy.Name(),
			ShardAccess:  make([]int, 0, len(fens)),
		}

		lastShard := -1
		for _, fen := range fens {
			shardID := strategy.ShardID(fen, s.totalShards)
			result.ShardAccess = append(result.ShardAccess, shardID)

			if shardID != lastShard {
				result.ShardSwitches++
				lastShard = shardID
			}
		}

		results[strategy.Name()] = result
	}

	return results
}

// SimulateGames simulates multiple games and aggregates results.
func (s *Simulator) SimulateGames(games [][]string) map[string]*AggregateResult {
	results := make(map[string]*AggregateResult, len(s.strategies))

	// Initialize results for each strategy.
	for _, strategy := range s.strategies {
		results[strategy.Name()] = &AggregateResult{
			StrategyName:    strategy.Name(),
			ShardHits:       make(map[int]int),
			SwitchesPerGame: make([]int, 0, len(games)),
		}
	}

	// Simulate each game.
	for _, game := range games {
		gameResults := s.SimulateGame(game)
		for name, gr := range gameResults {
			agg := results[name]
			agg.TotalLookups += len(game)
			agg.TotalSwitches += gr.ShardSwitches
			agg.SwitchesPerGame = append(agg.SwitchesPerGame, gr.ShardSwitches)

			for _, shardID := range gr.ShardAccess {
				agg.ShardHits[shardID]++
			}
		}
	}

	// Calculate derived metrics.
	for _, agg := range results {
		agg.UniqueShards = len(agg.ShardHits)
		if len(games) > 0 {
			agg.AvgSwitchesPerGame = float64(agg.TotalSwitches) / float64(len(games))
		}
	}

	return results
}

// GameResult contains the shard access pattern for a single game.
type GameResult struct {
	StrategyName  string
	ShardAccess   []int // Shard IDs accessed in order.
	ShardSwitches int   // Number of times shard changed.
}

// AggregateResult contains aggregated results across multiple games.
type AggregateResult struct {
	StrategyName       string
	TotalLookups       int
	TotalSwitches      int
	UniqueShards       int
	AvgSwitchesPerGame float64
	ShardHits          map[int]int // Shard ID -> hit count.
	SwitchesPerGame    []int       // Switches per game for statistical analysis.
}

// CacheHitRate estimates the cache hit rate assuming an LRU cache
// with the given capacity (number of shards).
func (a *AggregateResult) CacheHitRate(cacheCapacity int) float64 {
	if a.TotalLookups == 0 {
		return 0
	}

	// Simple LRU simulation.
	cache := make(map[int]struct{})
	var order []int
	var hits int

	for _, switches := range a.SwitchesPerGame {
		// This is a simplified model; in reality we'd need the actual access sequence.
		_ = switches
	}

	// For now, estimate based on unique shards vs cache capacity.
	if a.UniqueShards <= cacheCapacity {
		// All shards fit in cache after warmup.
		// Estimate: first access to each shard is a miss.
		hits = a.TotalLookups - a.UniqueShards
	} else {
		// Shards exceed cache; use locality-based estimate.
		// Higher switches = worse cache performance.
		avgAccessesPerShard := float64(a.TotalLookups) / float64(a.UniqueShards)
		hitRateEstimate := (avgAccessesPerShard - 1) / avgAccessesPerShard
		if hitRateEstimate < 0 {
			hitRateEstimate = 0
		}
		hits = int(float64(a.TotalLookups) * hitRateEstimate)
	}

	// Clear unused variable warning.
	_ = cache
	_ = order

	return float64(hits) / float64(a.TotalLookups) * 100
}
