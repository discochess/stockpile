package materialshard

import (
	"testing"
)

func TestStrategy_Name(t *testing.T) {
	s := New()
	if got := s.Name(); got != "material" {
		t.Errorf("Name() = %q, want %q", got, "material")
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
			name: "starting position black to move",
			fen:  "rnbqkbnr/pppppppp/8/8/4P3/8/PPPP1PPP/RNBQKBNR b KQkq e3 0 1",
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

func TestStrategy_ShardID_SameMaterialClusters(t *testing.T) {
	s := New()
	totalShards := 32768

	// Positions with same material should have same shard ID (modulo side to move).
	// These are both starting positions with all pieces.
	fen1 := "rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 1"
	fen2 := "rnbqkb1r/pppppppp/5n2/8/4P3/8/PPPP1PPP/RNBQKBNR w KQkq - 1 2" // Same material, different position

	id1 := s.ShardID(fen1, totalShards)
	id2 := s.ShardID(fen2, totalShards)

	// They should be the same because material is the same and both white to move.
	if id1 != id2 {
		t.Errorf("Same material positions should cluster: got shard %d and %d", id1, id2)
	}
}

func TestStrategy_ShardID_SideToMoveWithMoreShards(t *testing.T) {
	s := New()
	// With enough shards, side to move will produce different IDs.
	totalShards := 1 << 19 // 524288 shards, more than 2^18

	fenWhite := "rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 1"
	fenBlack := "rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR b KQkq - 0 1"

	idWhite := s.ShardID(fenWhite, totalShards)
	idBlack := s.ShardID(fenBlack, totalShards)

	// With enough shards, side to move bit (bit 18) is preserved.
	if idWhite == idBlack {
		t.Errorf("With %d shards, different side to move should produce different shards, both got %d", totalShards, idWhite)
	}
}

func TestStrategy_ShardID_SideToMoveClustersWithFewerShards(t *testing.T) {
	s := New()
	// With default 32768 shards, side to move bit gets lost in modulo.
	// This is actually desirable - positions with same material cluster together.
	totalShards := 32768

	fenWhite := "rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 1"
	fenBlack := "rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR b KQkq - 0 1"

	idWhite := s.ShardID(fenWhite, totalShards)
	idBlack := s.ShardID(fenBlack, totalShards)

	// With fewer shards, they may cluster together (which is fine).
	// Just verify both are valid shard IDs.
	if idWhite < 0 || idWhite >= totalShards {
		t.Errorf("White shard ID %d out of range [0, %d)", idWhite, totalShards)
	}
	if idBlack < 0 || idBlack >= totalShards {
		t.Errorf("Black shard ID %d out of range [0, %d)", idBlack, totalShards)
	}
}

func TestStrategy_ShardID_InvalidFEN(t *testing.T) {
	s := New()
	totalShards := 32768

	// Invalid FEN should still return a valid shard ID (via fallback hash).
	invalidFEN := "not a valid fen"
	id := s.ShardID(invalidFEN, totalShards)

	if id < 0 || id >= totalShards {
		t.Errorf("ShardID() for invalid FEN = %d, want 0 <= id < %d", id, totalShards)
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

func BenchmarkStrategy_ShardID_Endgame(b *testing.B) {
	s := New()
	fen := "8/8/8/4k3/8/8/4K3/4R3 w - - 0 1"
	totalShards := 32768

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s.ShardID(fen, totalShards)
	}
}
