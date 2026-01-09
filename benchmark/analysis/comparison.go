package analysis

import (
	"fmt"

	"github.com/discochess/stockpile/benchmark/simulation"
)

// StrategyComparison contains a full statistical comparison between two strategies.
type StrategyComparison struct {
	Strategy1       string
	Strategy2       string
	Stats1          *DescriptiveStats
	Stats2          *DescriptiveStats
	MannWhitney     *MannWhitneyResult
	EffectSize      *EffectSize
	BootstrapCI     *BootstrapResult
	Winner          string // Name of strategy with fewer switches, or "tie".
	WinnerConfident bool   // True if statistically significant.
}

// CompareStrategies performs a full statistical comparison between two strategies.
func CompareStrategies(
	result1, result2 *simulation.AggregateResult,
	bootstrapIterations int,
	confidence float64,
) *StrategyComparison {
	// Convert to float64 for statistical functions.
	sample1 := intsToFloats(result1.SwitchesPerGame)
	sample2 := intsToFloats(result2.SwitchesPerGame)

	mw := MannWhitneyU(sample1, sample2)
	es := ComputeEffectSize(sample1, sample2)
	bs := BootstrapConfidenceInterval(sample1, sample2, bootstrapIterations, confidence)

	// Determine winner.
	stats1 := Describe(sample1)
	stats2 := Describe(sample2)

	var winner string
	var confident bool

	if stats1.Mean < stats2.Mean {
		winner = result1.StrategyName
		confident = mw.Significant
	} else if stats2.Mean < stats1.Mean {
		winner = result2.StrategyName
		confident = mw.Significant
	} else {
		winner = "tie"
		confident = false
	}

	return &StrategyComparison{
		Strategy1:       result1.StrategyName,
		Strategy2:       result2.StrategyName,
		Stats1:          stats1,
		Stats2:          stats2,
		MannWhitney:     mw,
		EffectSize:      es,
		BootstrapCI:     bs,
		Winner:          winner,
		WinnerConfident: confident,
	}
}

// Summary returns a human-readable summary of the comparison.
func (c *StrategyComparison) Summary() string {
	sig := "not statistically significant"
	if c.MannWhitney.Significant {
		sig = fmt.Sprintf("statistically significant (p=%.4f)", c.MannWhitney.PValue)
	}

	return fmt.Sprintf(
		"%s vs %s:\n"+
			"  %s: mean=%.2f, median=%.2f, std=%.2f\n"+
			"  %s: mean=%.2f, median=%.2f, std=%.2f\n"+
			"  Difference: %.2f switches/game (%.1f%%)\n"+
			"  Effect size: %.2f (%s)\n"+
			"  Result: %s, %s",
		c.Strategy1, c.Strategy2,
		c.Strategy1, c.Stats1.Mean, c.Stats1.Median, c.Stats1.StdDev,
		c.Strategy2, c.Stats2.Mean, c.Stats2.Median, c.Stats2.StdDev,
		c.Stats1.Mean-c.Stats2.Mean,
		safePctDiff(c.Stats1.Mean, c.Stats2.Mean),
		c.EffectSize.CohensD, c.EffectSize.Interpretation,
		c.Winner, sig,
	)
}

func intsToFloats(ints []int) []float64 {
	floats := make([]float64, len(ints))
	for i, v := range ints {
		floats[i] = float64(v)
	}
	return floats
}

func safePctDiff(a, b float64) float64 {
	if b == 0 {
		return 0
	}
	return (a - b) / b * 100
}

// MultiStrategyComparison compares multiple strategies against a baseline.
type MultiStrategyComparison struct {
	Baseline    string
	Comparisons []*StrategyComparison
}

// CompareAll compares all strategies against the first one (baseline).
func CompareAll(
	results map[string]*simulation.AggregateResult,
	baseline string,
	bootstrapIterations int,
	confidence float64,
) *MultiStrategyComparison {
	baseResult, ok := results[baseline]
	if !ok {
		return nil
	}

	multi := &MultiStrategyComparison{
		Baseline: baseline,
	}

	for name, result := range results {
		if name == baseline {
			continue
		}
		comp := CompareStrategies(baseResult, result, bootstrapIterations, confidence)
		multi.Comparisons = append(multi.Comparisons, comp)
	}

	return multi
}
