package search

import (
	"testing"
)

func TestSearch(t *testing.T) {
	// Sample sorted JSONL data (sorted by FEN).
	data := []byte(`{"fen":"8/8/8/4k3/8/8/4K3/4R3 w - -","evals":[{"pvs":[{"cp":500,"line":"Re1+ Kd4"}],"knodes":1000,"depth":20}]}
{"fen":"r1bqkbnr/pppp1ppp/2n5/4p3/4P3/5N2/PPPP1PPP/RNBQKB1R w KQkq -","evals":[{"pvs":[{"cp":25,"line":"Bb5 a6"}],"knodes":2000,"depth":25}]}
{"fen":"rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq -","evals":[{"pvs":[{"cp":20,"line":"e4 e5"}],"knodes":3000,"depth":30}]}
`)

	tests := []struct {
		name    string
		fen     string
		wantCP  int
		wantErr bool
	}{
		{
			name:   "find starting position",
			fen:    "rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq -",
			wantCP: 20,
		},
		{
			name:   "find endgame position",
			fen:    "8/8/8/4k3/8/8/4K3/4R3 w - -",
			wantCP: 500,
		},
		{
			name:   "find middlegame position",
			fen:    "r1bqkbnr/pppp1ppp/2n5/4p3/4P3/5N2/PPPP1PPP/RNBQKB1R w KQkq -",
			wantCP: 25,
		},
		{
			name:    "not found",
			fen:     "8/8/8/8/8/8/8/4K2k w - -",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			record, err := Search(data, tt.fen)
			if (err != nil) != tt.wantErr {
				t.Errorf("Search() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr {
				return
			}

			if record.FEN != tt.fen {
				t.Errorf("FEN = %q, want %q", record.FEN, tt.fen)
			}

			if len(record.Evals) == 0 || len(record.Evals[0].PVs) == 0 {
				t.Fatal("No evaluations found")
			}

			if record.Evals[0].PVs[0].CP == nil {
				t.Fatal("CP is nil")
			}

			if *record.Evals[0].PVs[0].CP != tt.wantCP {
				t.Errorf("CP = %d, want %d", *record.Evals[0].PVs[0].CP, tt.wantCP)
			}
		})
	}
}

func TestSearch_EmptyData(t *testing.T) {
	_, err := Search([]byte{}, "rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq -")
	if err != ErrNotFound {
		t.Errorf("Search() error = %v, want ErrNotFound", err)
	}
}

func TestExtractFEN(t *testing.T) {
	tests := []struct {
		name string
		line string
		want string
	}{
		{
			name: "valid json",
			line: `{"fen":"rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq -","evals":[]}`,
			want: "rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq -",
		},
		{
			name: "endgame position",
			line: `{"fen":"8/8/8/4k3/8/8/4K3/4R3 w - -","evals":[]}`,
			want: "8/8/8/4k3/8/8/4K3/4R3 w - -",
		},
		{
			name: "no fen field",
			line: `{"other":"value"}`,
			want: "",
		},
		{
			name: "malformed",
			line: `not json`,
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractFEN([]byte(tt.line))
			if got != tt.want {
				t.Errorf("extractFEN() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestSplitLines(t *testing.T) {
	tests := []struct {
		name  string
		data  string
		count int
	}{
		{
			name:  "single line",
			data:  "line1",
			count: 1,
		},
		{
			name:  "multiple lines",
			data:  "line1\nline2\nline3",
			count: 3,
		},
		{
			name:  "trailing newline",
			data:  "line1\nline2\n",
			count: 2,
		},
		{
			name:  "empty lines filtered",
			data:  "line1\n\nline2\n\n",
			count: 2,
		},
		{
			name:  "empty data",
			data:  "",
			count: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lines := splitLines([]byte(tt.data))
			if len(lines) != tt.count {
				t.Errorf("splitLines() returned %d lines, want %d", len(lines), tt.count)
			}
		})
	}
}

func BenchmarkSearch(b *testing.B) {
	// Generate sorted JSONL data with 1000 entries.
	var data []byte
	for i := 0; i < 1000; i++ {
		// Create FENs that sort lexicographically.
		fen := "position" + string(rune('A'+i/26)) + string(rune('A'+i%26))
		line := `{"fen":"` + fen + `","evals":[{"pvs":[{"cp":0,"line":"e4"}],"knodes":1000,"depth":20}]}` + "\n"
		data = append(data, line...)
	}

	targetFEN := "positionMN" // Somewhere in the middle.

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = Search(data, targetFEN)
	}
}

func BenchmarkExtractFEN(b *testing.B) {
	line := []byte(`{"fen":"rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq -","evals":[{"pvs":[{"cp":20,"line":"e4 e5 Nf3"}],"knodes":3000,"depth":30}]}`)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		extractFEN(line)
	}
}
