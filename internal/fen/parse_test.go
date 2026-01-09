package fen

import (
	"testing"
)

func TestNormalize(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{
			name:  "starting position",
			input: "rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 1",
			want:  "rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq -",
		},
		{
			name:  "position after e4",
			input: "rnbqkbnr/pppppppp/8/8/4P3/8/PPPP1PPP/RNBQKBNR b KQkq e3 0 1",
			want:  "rnbqkbnr/pppppppp/8/8/4P3/8/PPPP1PPP/RNBQKBNR b KQkq e3",
		},
		{
			name:  "no castling rights",
			input: "r3k2r/pppppppp/8/8/8/8/PPPPPPPP/R3K2R w - - 10 20",
			want:  "r3k2r/pppppppp/8/8/8/8/PPPPPPPP/R3K2R w - -",
		},
		{
			name:  "complex middlegame",
			input: "r1bqkb1r/pppp1ppp/2n2n2/4p3/2B1P3/5N2/PPPP1PPP/RNBQK2R w KQkq - 4 4",
			want:  "r1bqkb1r/pppp1ppp/2n2n2/4p3/2B1P3/5N2/PPPP1PPP/RNBQK2R w KQkq -",
		},
		{
			name:    "empty string",
			input:   "",
			wantErr: true,
		},
		{
			name:    "too few fields",
			input:   "rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w",
			wantErr: true,
		},
		{
			name:    "invalid side to move",
			input:   "rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR x KQkq - 0 1",
			wantErr: true,
		},
		{
			name:    "invalid piece placement - wrong rank count",
			input:   "rnbqkbnr/pppppppp/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 1",
			wantErr: true,
		},
		{
			name:    "invalid piece placement - wrong square count",
			input:   "rnbqkbnr/ppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 1",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Normalize(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("Normalize() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("Normalize() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestParseMaterial(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    Material
		wantErr bool
	}{
		{
			name:  "starting position",
			input: "rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 1",
			want: Material{
				WhitePawns: 8, WhiteKnights: 2, WhiteBishops: 2, WhiteRooks: 2, WhiteQueens: 1,
				BlackPawns: 8, BlackKnights: 2, BlackBishops: 2, BlackRooks: 2, BlackQueens: 1,
			},
		},
		{
			name:  "king and rook endgame",
			input: "8/8/8/4k3/8/8/4K3/4R3 w - - 0 1",
			want: Material{
				WhiteRooks: 1,
			},
		},
		{
			name:  "queen vs two rooks",
			input: "8/8/8/4k3/8/8/4K3/Q3RR2 w - - 0 1",
			want: Material{
				WhiteQueens: 1,
				WhiteRooks:  2,
			},
		},
		{
			name:  "multiple queens (promotion)",
			input: "QQQQk3/8/8/8/8/8/8/4K3 w - - 0 1",
			want: Material{
				WhiteQueens: 4,
			},
		},
		{
			name:    "empty string",
			input:   "",
			wantErr: true,
		},
		{
			name:    "invalid piece character",
			input:   "rnbxkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 1",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseMaterial(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseMaterial() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("ParseMaterial() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestSideToMove(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{
			name:  "white to move",
			input: "rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 1",
			want:  "w",
		},
		{
			name:  "black to move",
			input: "rnbqkbnr/pppppppp/8/8/4P3/8/PPPP1PPP/RNBQKBNR b KQkq e3 0 1",
			want:  "b",
		},
		{
			name:    "invalid side",
			input:   "rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR x KQkq - 0 1",
			wantErr: true,
		},
		{
			name:    "missing side",
			input:   "rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := SideToMove(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("SideToMove() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("SideToMove() = %q, want %q", got, tt.want)
			}
		})
	}
}

func BenchmarkNormalize(b *testing.B) {
	fen := "rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 1"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = Normalize(fen)
	}
}

func BenchmarkParseMaterial(b *testing.B) {
	fen := "rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 1"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = ParseMaterial(fen)
	}
}
