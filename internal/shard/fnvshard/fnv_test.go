package fnvshard

import (
	"testing"
)

func TestStrategy_Name(t *testing.T) {
	s := New()
	if got := s.Name(); got != "fnv32" {
		t.Errorf("Name() = %q, want %q", got, "fnv32")
	}
}

func TestStrategy_ShardID(t *testing.T) {
	s := New()
	totalShards := 32768

	tests := []struct {
		name string
		fen  string
	}{
		{
			name: "starting position",
			fen:  "rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 1",
		},
		{
			name: "endgame K+R vs K",
			fen:  "8/8/8/4k3/8/8/4K3/4R3 w - - 0 1",
		},
		{
			name: "complex middlegame",
			fen:  "r1bqkb1r/pppp1ppp/2n2n2/4p3/2B1P3/5N2/PPPP1PPP/RNBQK2R w KQkq - 4 4",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			id := s.ShardID(tt.fen, totalShards)
			if id < 0 || id >= totalShards {
				t.Errorf("ShardID() = %d, want 0 <= id < %d", id, totalShards)
			}
		})
	}
}

func TestStrategy_ShardID_Consistency(t *testing.T) {
	s := New()
	totalShards := 32768
	fen := "rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 1"

	// Same FEN should always produce same shard ID.
	id1 := s.ShardID(fen, totalShards)
	id2 := s.ShardID(fen, totalShards)

	if id1 != id2 {
		t.Errorf("ShardID() not consistent: got %d and %d", id1, id2)
	}
}

func TestStrategy_ShardID_Distribution(t *testing.T) {
	s := New()
	totalShards := 256

	// Test that different FENs produce different shard IDs (mostly).
	fens := []string{
		"rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 1",
		"rnbqkbnr/pppppppp/8/8/4P3/8/PPPP1PPP/RNBQKBNR b KQkq e3 0 1",
		"rnbqkbnr/pppp1ppp/8/4p3/4P3/8/PPPP1PPP/RNBQKBNR w KQkq e6 0 2",
		"rnbqkbnr/pppp1ppp/8/4p3/4P3/5N2/PPPP1PPP/RNBQKB1R b KQkq - 1 2",
		"r1bqkbnr/pppp1ppp/2n5/4p3/4P3/5N2/PPPP1PPP/RNBQKB1R w KQkq - 2 3",
	}

	shardIDs := make(map[int]bool, len(fens))
	for _, fen := range fens {
		id := s.ShardID(fen, totalShards)
		shardIDs[id] = true
	}

	// With FNV hash, we expect mostly unique shard IDs.
	if len(shardIDs) < len(fens)-1 {
		t.Errorf("FNV hash should produce mostly unique shards, got %d unique out of %d", len(shardIDs), len(fens))
	}
}

func BenchmarkStrategy_ShardID(b *testing.B) {
	s := New()
	fen := "rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 1"
	totalShards := 32768

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s.ShardID(fen, totalShards)
	}
}
