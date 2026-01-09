package simulation

import (
	"sort"
)

// Metrics contains computed metrics from simulation results.
type Metrics struct {
	// Core metrics.
	TotalLookups       int
	TotalSwitches      int
	UniqueShards       int
	AvgSwitchesPerGame float64

	// Distribution metrics.
	MedianSwitchesPerGame float64
	P90SwitchesPerGame    float64
	P99SwitchesPerGame    float64
	MinSwitchesPerGame    int
	MaxSwitchesPerGame    int

	// Locality metrics.
	ShardConcentration float64 // Gini coefficient of shard usage.
	TopShardPct        float64 // Percentage of lookups in top 10% of shards.
}

// ComputeMetrics computes detailed metrics from aggregate results.
func ComputeMetrics(result *AggregateResult) *Metrics {
	m := &Metrics{
		TotalLookups:       result.TotalLookups,
		TotalSwitches:      result.TotalSwitches,
		UniqueShards:       result.UniqueShards,
		AvgSwitchesPerGame: result.AvgSwitchesPerGame,
	}

	if len(result.SwitchesPerGame) > 0 {
		// Sort for percentile calculation.
		sorted := make([]int, len(result.SwitchesPerGame))
		copy(sorted, result.SwitchesPerGame)
		sort.Ints(sorted)

		m.MinSwitchesPerGame = sorted[0]
		m.MaxSwitchesPerGame = sorted[len(sorted)-1]
		m.MedianSwitchesPerGame = percentile(sorted, 50)
		m.P90SwitchesPerGame = percentile(sorted, 90)
		m.P99SwitchesPerGame = percentile(sorted, 99)
	}

	// Compute shard concentration (Gini coefficient).
	if len(result.ShardHits) > 0 {
		m.ShardConcentration = computeGini(result.ShardHits)
		m.TopShardPct = computeTopShardPct(result.ShardHits, result.TotalLookups, 0.1)
	}

	return m
}

func percentile(sorted []int, p float64) float64 {
	if len(sorted) == 0 {
		return 0
	}
	idx := int(float64(len(sorted)-1) * p / 100)
	if idx >= len(sorted) {
		idx = len(sorted) - 1
	}
	return float64(sorted[idx])
}

func computeGini(hits map[int]int) float64 {
	if len(hits) == 0 {
		return 0
	}

	// Extract values and sort.
	values := make([]int, 0, len(hits))
	for _, v := range hits {
		values = append(values, v)
	}
	sort.Ints(values)

	n := float64(len(values))
	var sum, cumulativeSum float64
	for i, v := range values {
		sum += float64(v)
		cumulativeSum += float64(i+1) * float64(v)
	}

	if sum == 0 {
		return 0
	}

	// Gini coefficient formula.
	return (2*cumulativeSum)/(n*sum) - (n+1)/n
}

func computeTopShardPct(hits map[int]int, total int, topFraction float64) float64 {
	if total == 0 || len(hits) == 0 {
		return 0
	}

	// Sort by hit count descending.
	type shardHit struct {
		shard int
		hits  int
	}
	sorted := make([]shardHit, 0, len(hits))
	for s, h := range hits {
		sorted = append(sorted, shardHit{shard: s, hits: h})
	}
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].hits > sorted[j].hits
	})

	// Sum hits from top shards.
	topCount := int(float64(len(sorted)) * topFraction)
	if topCount < 1 {
		topCount = 1
	}

	var topHits int
	for i := 0; i < topCount && i < len(sorted); i++ {
		topHits += sorted[i].hits
	}

	return float64(topHits) / float64(total) * 100
}

// CompareMetrics compares metrics between two strategies.
type MetricsComparison struct {
	Strategy1 string
	Strategy2 string

	SwitchesDiff       float64 // Positive means Strategy1 has more switches.
	SwitchesDiffPct    float64
	ConcentrationDiff  float64
	TopShardPctDiff    float64
	UniqueShardsDiff   int
}

// Compare compares two metrics and returns the differences.
func Compare(m1, m2 *Metrics, name1, name2 string) *MetricsComparison {
	return &MetricsComparison{
		Strategy1:          name1,
		Strategy2:          name2,
		SwitchesDiff:       m1.AvgSwitchesPerGame - m2.AvgSwitchesPerGame,
		SwitchesDiffPct:    safeDiffPct(m1.AvgSwitchesPerGame, m2.AvgSwitchesPerGame),
		ConcentrationDiff:  m1.ShardConcentration - m2.ShardConcentration,
		TopShardPctDiff:    m1.TopShardPct - m2.TopShardPct,
		UniqueShardsDiff:   m1.UniqueShards - m2.UniqueShards,
	}
}

func safeDiffPct(a, b float64) float64 {
	if b == 0 {
		return 0
	}
	return (a - b) / b * 100
}
