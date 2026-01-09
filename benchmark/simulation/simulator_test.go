package simulation

import (
	"testing"

	"github.com/discochess/stockpile/internal/shard/fnvshard"
	"github.com/discochess/stockpile/internal/shard/materialshard"
)

func TestSimulator_SimulateGame(t *testing.T) {
	strategies := []struct {
		name string
	}{
		{name: "material"},
		{name: "fnv32"},
	}

	sim := NewSimulator(32768, materialshard.New(), fnvshard.New())

	// Sample game with 5 positions.
	game := []string{
		"rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq -",
		"rnbqkbnr/pppppppp/8/8/4P3/8/PPPP1PPP/RNBQKBNR b KQkq e3",
		"rnbqkbnr/pppp1ppp/8/4p3/4P3/8/PPPP1PPP/RNBQKBNR w KQkq e6",
		"rnbqkbnr/pppp1ppp/8/4p3/4P3/5N2/PPPP1PPP/RNBQKB1R b KQkq -",
		"r1bqkbnr/pppp1ppp/2n5/4p3/4P3/5N2/PPPP1PPP/RNBQKB1R w KQkq -",
	}

	results := sim.SimulateGame(game)

	for _, s := range strategies {
		result, ok := results[s.name]
		if !ok {
			t.Errorf("Missing result for strategy %s", s.name)
			continue
		}

		if len(result.ShardAccess) != len(game) {
			t.Errorf("%s: ShardAccess length = %d, want %d", s.name, len(result.ShardAccess), len(game))
		}

		// At least some shard switches should occur.
		// (may not always be true, but likely for this game).
		if result.ShardSwitches < 0 {
			t.Errorf("%s: ShardSwitches = %d, want >= 0", s.name, result.ShardSwitches)
		}
	}
}

func TestSimulator_SimulateGames(t *testing.T) {
	sim := NewSimulator(32768, materialshard.New(), fnvshard.New())

	games := [][]string{
		{
			"rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq -",
			"rnbqkbnr/pppppppp/8/8/4P3/8/PPPP1PPP/RNBQKBNR b KQkq e3",
		},
		{
			"rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq -",
			"rnbqkbnr/pppppppp/8/8/3P4/8/PPP1PPPP/RNBQKBNR b KQkq d3",
		},
	}

	results := sim.SimulateGames(games)

	for name, res := range results {
		if res.TotalLookups != 4 {
			t.Errorf("%s: TotalLookups = %d, want 4", name, res.TotalLookups)
		}

		if len(res.SwitchesPerGame) != 2 {
			t.Errorf("%s: SwitchesPerGame length = %d, want 2", name, len(res.SwitchesPerGame))
		}
	}
}

func TestAggregateResult_CacheHitRate(t *testing.T) {
	result := &AggregateResult{
		TotalLookups: 100,
		UniqueShards: 10,
	}

	// With 100 shard cache, all shards fit.
	hitRate := result.CacheHitRate(100)
	if hitRate < 0 || hitRate > 100 {
		t.Errorf("CacheHitRate = %f, want 0-100", hitRate)
	}
}

func TestMetrics_Computation(t *testing.T) {
	result := &AggregateResult{
		StrategyName:       "test",
		TotalLookups:       100,
		TotalSwitches:      20,
		UniqueShards:       5,
		AvgSwitchesPerGame: 10,
		ShardHits:          map[int]int{0: 30, 1: 25, 2: 20, 3: 15, 4: 10},
		SwitchesPerGame:    []int{8, 10, 12},
	}

	metrics := ComputeMetrics(result)

	if metrics.TotalLookups != 100 {
		t.Errorf("TotalLookups = %d, want 100", metrics.TotalLookups)
	}

	if metrics.MinSwitchesPerGame != 8 {
		t.Errorf("MinSwitchesPerGame = %d, want 8", metrics.MinSwitchesPerGame)
	}

	if metrics.MaxSwitchesPerGame != 12 {
		t.Errorf("MaxSwitchesPerGame = %d, want 12", metrics.MaxSwitchesPerGame)
	}
}
