// Package analysis provides statistical analysis for benchmark results.
package analysis

import (
	"math"
	"sort"

	"gonum.org/v1/gonum/stat"
)

// MannWhitneyResult contains the result of a Mann-Whitney U test.
type MannWhitneyResult struct {
	U          float64 // U statistic.
	Z          float64 // Z score (normal approximation).
	PValue     float64 // Two-tailed p-value.
	Significant bool   // True if p < 0.05.
}

// MannWhitneyU performs the Mann-Whitney U test on two samples.
// This is a non-parametric test to determine if two samples come from
// different distributions.
func MannWhitneyU(sample1, sample2 []float64) *MannWhitneyResult {
	n1 := float64(len(sample1))
	n2 := float64(len(sample2))

	if n1 == 0 || n2 == 0 {
		return &MannWhitneyResult{}
	}

	// Combine and rank all values.
	type rankedValue struct {
		value  float64
		sample int // 1 or 2
	}

	combined := make([]rankedValue, 0, int(n1+n2))
	for _, v := range sample1 {
		combined = append(combined, rankedValue{value: v, sample: 1})
	}
	for _, v := range sample2 {
		combined = append(combined, rankedValue{value: v, sample: 2})
	}

	sort.Slice(combined, func(i, j int) bool {
		return combined[i].value < combined[j].value
	})

	// Assign ranks (handling ties).
	ranks := make([]float64, len(combined))
	i := 0
	for i < len(combined) {
		j := i
		for j < len(combined) && combined[j].value == combined[i].value {
			j++
		}
		avgRank := float64(i+j+1) / 2
		for k := i; k < j; k++ {
			ranks[k] = avgRank
		}
		i = j
	}

	// Sum ranks for sample 1.
	var r1 float64
	for i, rv := range combined {
		if rv.sample == 1 {
			r1 += ranks[i]
		}
	}

	// Calculate U statistic.
	u1 := r1 - n1*(n1+1)/2
	u2 := n1*n2 - u1
	u := math.Min(u1, u2)

	// Normal approximation for large samples.
	mu := n1 * n2 / 2
	sigma := math.Sqrt(n1 * n2 * (n1 + n2 + 1) / 12)

	z := 0.0
	if sigma > 0 {
		z = (u - mu) / sigma
	}

	// Two-tailed p-value using normal approximation.
	pValue := 2 * normalCDF(-math.Abs(z))

	return &MannWhitneyResult{
		U:          u,
		Z:          z,
		PValue:     pValue,
		Significant: pValue < 0.05,
	}
}

// normalCDF computes the cumulative distribution function of the standard normal.
func normalCDF(x float64) float64 {
	return 0.5 * (1 + math.Erf(x/math.Sqrt2))
}

// EffectSize contains effect size metrics.
type EffectSize struct {
	CohensD     float64 // Cohen's d: (mean1 - mean2) / pooled_std.
	Interpretation string // "negligible", "small", "medium", "large".
}

// ComputeEffectSize computes Cohen's d effect size.
func ComputeEffectSize(sample1, sample2 []float64) *EffectSize {
	if len(sample1) == 0 || len(sample2) == 0 {
		return &EffectSize{Interpretation: "undefined"}
	}

	mean1 := stat.Mean(sample1, nil)
	mean2 := stat.Mean(sample2, nil)
	std1 := stat.StdDev(sample1, nil)
	std2 := stat.StdDev(sample2, nil)

	// Pooled standard deviation.
	n1 := float64(len(sample1))
	n2 := float64(len(sample2))
	pooledVar := ((n1-1)*std1*std1 + (n2-1)*std2*std2) / (n1 + n2 - 2)
	pooledStd := math.Sqrt(pooledVar)

	var d float64
	if pooledStd > 0 {
		d = (mean1 - mean2) / pooledStd
	}

	return &EffectSize{
		CohensD:        d,
		Interpretation: interpretCohensD(math.Abs(d)),
	}
}

func interpretCohensD(d float64) string {
	switch {
	case d < 0.2:
		return "negligible"
	case d < 0.5:
		return "small"
	case d < 0.8:
		return "medium"
	default:
		return "large"
	}
}

// BootstrapCI computes a bootstrap confidence interval for the mean difference.
type BootstrapResult struct {
	MeanDiff   float64
	LowerBound float64
	UpperBound float64
	Confidence float64 // e.g., 0.95 for 95% CI.
}

// BootstrapConfidenceInterval computes a confidence interval using bootstrap.
func BootstrapConfidenceInterval(sample1, sample2 []float64, iterations int, confidence float64) *BootstrapResult {
	if len(sample1) == 0 || len(sample2) == 0 {
		return &BootstrapResult{Confidence: confidence}
	}

	// Actual mean difference.
	mean1 := stat.Mean(sample1, nil)
	mean2 := stat.Mean(sample2, nil)
	actualDiff := mean1 - mean2

	// Bootstrap resampling.
	diffs := make([]float64, iterations)
	for i := 0; i < iterations; i++ {
		resample1 := resample(sample1)
		resample2 := resample(sample2)
		diffs[i] = stat.Mean(resample1, nil) - stat.Mean(resample2, nil)
	}

	sort.Float64s(diffs)

	// Percentile method for CI.
	alpha := 1 - confidence
	lowerIdx := int(alpha / 2 * float64(iterations))
	upperIdx := int((1 - alpha/2) * float64(iterations))

	if lowerIdx < 0 {
		lowerIdx = 0
	}
	if upperIdx >= iterations {
		upperIdx = iterations - 1
	}

	return &BootstrapResult{
		MeanDiff:   actualDiff,
		LowerBound: diffs[lowerIdx],
		UpperBound: diffs[upperIdx],
		Confidence: confidence,
	}
}

// resample performs bootstrap resampling with replacement.
func resample(sample []float64) []float64 {
	n := len(sample)
	result := make([]float64, n)
	for i := 0; i < n; i++ {
		result[i] = sample[pseudoRandom(i, n)%n]
	}
	return result
}

// pseudoRandom generates a deterministic pseudo-random index for reproducibility.
func pseudoRandom(seed, n int) int {
	// Simple LCG for reproducible results.
	return (seed*1103515245 + 12345) % (1 << 31) % n
}

// DescriptiveStats contains basic descriptive statistics.
type DescriptiveStats struct {
	N      int
	Mean   float64
	Median float64
	StdDev float64
	Min    float64
	Max    float64
	P25    float64
	P75    float64
}

// Describe computes descriptive statistics for a sample.
func Describe(sample []float64) *DescriptiveStats {
	if len(sample) == 0 {
		return &DescriptiveStats{}
	}

	sorted := make([]float64, len(sample))
	copy(sorted, sample)
	sort.Float64s(sorted)

	return &DescriptiveStats{
		N:      len(sample),
		Mean:   stat.Mean(sample, nil),
		Median: sorted[len(sorted)/2],
		StdDev: stat.StdDev(sample, nil),
		Min:    sorted[0],
		Max:    sorted[len(sorted)-1],
		P25:    percentileFloat(sorted, 25),
		P75:    percentileFloat(sorted, 75),
	}
}

func percentileFloat(sorted []float64, p float64) float64 {
	idx := int(float64(len(sorted)-1) * p / 100)
	if idx >= len(sorted) {
		idx = len(sorted) - 1
	}
	return sorted[idx]
}
