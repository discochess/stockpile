package stockpile

import "testing"

func TestEval_BestPV(t *testing.T) {
	tests := []struct {
		name    string
		eval    Eval
		wantNil bool
	}{
		{
			name:    "empty PVs",
			eval:    Eval{},
			wantNil: true,
		},
		{
			name: "single PV",
			eval: Eval{
				PVs: []PV{{Centipawns: intPtr(100)}},
			},
			wantNil: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.eval.BestPV()
			if (got == nil) != tt.wantNil {
				t.Errorf("BestPV() = %v, wantNil = %v", got, tt.wantNil)
			}
		})
	}
}

func TestEval_IsMate(t *testing.T) {
	tests := []struct {
		name string
		eval Eval
		want bool
	}{
		{
			name: "no PVs",
			eval: Eval{},
			want: false,
		},
		{
			name: "not mate",
			eval: Eval{
				PVs: []PV{{Centipawns: intPtr(100)}},
			},
			want: false,
		},
		{
			name: "is mate",
			eval: Eval{
				PVs: []PV{{Mate: intPtr(3)}},
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.eval.IsMate(); got != tt.want {
				t.Errorf("IsMate() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPV_Score(t *testing.T) {
	tests := []struct {
		name string
		pv   PV
		want string
	}{
		{
			name: "mate in 3",
			pv:   PV{Mate: intPtr(3)},
			want: "#3",
		},
		{
			name: "mate in -5",
			pv:   PV{Mate: intPtr(-5)},
			want: "#-5",
		},
		{
			name: "positive centipawns",
			pv:   PV{Centipawns: intPtr(125)},
			want: "+1.25",
		},
		{
			name: "negative centipawns",
			pv:   PV{Centipawns: intPtr(-50)},
			want: "-0.50",
		},
		{
			name: "zero centipawns",
			pv:   PV{Centipawns: intPtr(0)},
			want: "+0.00",
		},
		{
			name: "small positive",
			pv:   PV{Centipawns: intPtr(5)},
			want: "+0.05",
		},
		{
			name: "nil centipawns",
			pv:   PV{},
			want: "?",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.pv.Score(); got != tt.want {
				t.Errorf("Score() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestPV_IsMate(t *testing.T) {
	tests := []struct {
		name string
		pv   PV
		want bool
	}{
		{
			name: "is mate",
			pv:   PV{Mate: intPtr(5)},
			want: true,
		},
		{
			name: "not mate",
			pv:   PV{Centipawns: intPtr(100)},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.pv.IsMate(); got != tt.want {
				t.Errorf("IsMate() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestEval_Score(t *testing.T) {
	tests := []struct {
		name string
		eval Eval
		want string
	}{
		{
			name: "no PVs",
			eval: Eval{},
			want: "?",
		},
		{
			name: "with PV",
			eval: Eval{
				PVs: []PV{{Centipawns: intPtr(200)}},
			},
			want: "+2.00",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.eval.Score(); got != tt.want {
				t.Errorf("Score() = %q, want %q", got, tt.want)
			}
		})
	}
}

func intPtr(i int) *int {
	return &i
}
