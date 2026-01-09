package analysis

import (
	"math"
	"testing"
)

func TestMannWhitneyU(t *testing.T) {
	tests := []struct {
		name        string
		sample1     []float64
		sample2     []float64
		wantSignif  bool
	}{
		{
			name:       "identical samples",
			sample1:    []float64{1, 2, 3, 4, 5},
			sample2:    []float64{1, 2, 3, 4, 5},
			wantSignif: false,
		},
		{
			name:       "clearly different samples",
			sample1:    []float64{1, 2, 3, 4, 5},
			sample2:    []float64{10, 11, 12, 13, 14},
			wantSignif: true,
		},
		{
			name:       "highly overlapping samples",
			sample1:    []float64{3, 4, 5, 6, 7},
			sample2:    []float64{4, 5, 6, 7, 8},
			wantSignif: false, // Highly overlapping, not significant.
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := MannWhitneyU(tt.sample1, tt.sample2)
			if result.Significant != tt.wantSignif {
				t.Errorf("Significant = %v, want %v (p=%f)", result.Significant, tt.wantSignif, result.PValue)
			}
		})
	}
}

func TestMannWhitneyU_Empty(t *testing.T) {
	result := MannWhitneyU([]float64{}, []float64{1, 2, 3})
	if result.U != 0 {
		t.Errorf("U = %f, want 0 for empty sample", result.U)
	}
}

func TestEffectSize(t *testing.T) {
	tests := []struct {
		name   string
		sample1 []float64
		sample2 []float64
		wantInterp string
	}{
		{
			name:       "large effect",
			sample1:    []float64{1, 2, 3, 4, 5},
			sample2:    []float64{10, 11, 12, 13, 14},
			wantInterp: "large",
		},
		{
			name:       "negligible effect",
			sample1:    []float64{5, 5, 5, 5, 5},
			sample2:    []float64{5.1, 5, 4.9, 5, 5},
			wantInterp: "negligible",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ComputeEffectSize(tt.sample1, tt.sample2)
			if result.Interpretation != tt.wantInterp {
				t.Errorf("Interpretation = %s, want %s (d=%f)", result.Interpretation, tt.wantInterp, result.CohensD)
			}
		})
	}
}

func TestDescribe(t *testing.T) {
	sample := []float64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}
	stats := Describe(sample)

	if stats.N != 10 {
		t.Errorf("N = %d, want 10", stats.N)
	}
	if stats.Mean != 5.5 {
		t.Errorf("Mean = %f, want 5.5", stats.Mean)
	}
	if stats.Min != 1 {
		t.Errorf("Min = %f, want 1", stats.Min)
	}
	if stats.Max != 10 {
		t.Errorf("Max = %f, want 10", stats.Max)
	}
}

func TestDescribe_Empty(t *testing.T) {
	stats := Describe([]float64{})
	if stats.N != 0 {
		t.Errorf("N = %d, want 0", stats.N)
	}
}

func TestBootstrapConfidenceInterval(t *testing.T) {
	sample1 := []float64{1, 2, 3, 4, 5}
	sample2 := []float64{6, 7, 8, 9, 10}

	result := BootstrapConfidenceInterval(sample1, sample2, 1000, 0.95)

	// Mean difference should be -5 (1-6 avg).
	if math.Abs(result.MeanDiff-(-5)) > 0.1 {
		t.Errorf("MeanDiff = %f, want approximately -5", result.MeanDiff)
	}

	// Confidence interval should contain the true difference.
	if result.LowerBound > result.MeanDiff || result.UpperBound < result.MeanDiff {
		t.Errorf("CI [%f, %f] does not contain mean diff %f", result.LowerBound, result.UpperBound, result.MeanDiff)
	}
}
