// Package reporting provides report generation for benchmark results.
package reporting

import (
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/discochess/stockpile/benchmark/analysis"
	"github.com/discochess/stockpile/benchmark/simulation"
)

// MarkdownReport generates benchmark reports in Markdown format.
type MarkdownReport struct {
	w io.Writer
}

// NewMarkdownReport creates a new Markdown report writer.
func NewMarkdownReport(w io.Writer) *MarkdownReport {
	return &MarkdownReport{w: w}
}

// WriteHeader writes the report header.
func (r *MarkdownReport) WriteHeader(title string) {
	fmt.Fprintf(r.w, "# %s\n\n", title)
	fmt.Fprintf(r.w, "Generated: %s\n\n", time.Now().Format(time.RFC3339))
}

// WriteMethodology writes the methodology section.
func (r *MarkdownReport) WriteMethodology(gamesCount, positionsCount int) {
	fmt.Fprintln(r.w, "## Methodology")
	fmt.Fprintln(r.w)
	fmt.Fprintf(r.w, "- **Games analyzed:** %d\n", gamesCount)
	fmt.Fprintf(r.w, "- **Positions evaluated:** %d\n", positionsCount)
	fmt.Fprintln(r.w, "- **Metric:** Shard switches per game (lower is better)")
	fmt.Fprintln(r.w, "- **Statistical tests:** Mann-Whitney U (non-parametric), Cohen's d effect size")
	fmt.Fprintln(r.w)
}

// WriteSummaryTable writes the summary comparison table.
func (r *MarkdownReport) WriteSummaryTable(results map[string]*simulation.AggregateResult) {
	fmt.Fprintln(r.w, "## Summary")
	fmt.Fprintln(r.w)
	fmt.Fprintln(r.w, "| Strategy | Avg Switches | Median | Unique Shards | Est. Cache Hit Rate |")
	fmt.Fprintln(r.w, "|----------|--------------|--------|---------------|---------------------|")

	for name, res := range results {
		metrics := simulation.ComputeMetrics(res)
		cacheHitRate := res.CacheHitRate(100) // Assume 100-shard cache.
		fmt.Fprintf(r.w, "| %s | %.2f | %.0f | %d | %.1f%% |\n",
			name, metrics.AvgSwitchesPerGame, metrics.MedianSwitchesPerGame,
			metrics.UniqueShards, cacheHitRate)
	}
	fmt.Fprintln(r.w)
}

// WriteComparison writes a detailed comparison section.
func (r *MarkdownReport) WriteComparison(comp *analysis.StrategyComparison) {
	fmt.Fprintf(r.w, "## %s vs %s\n\n", comp.Strategy1, comp.Strategy2)

	// Statistics table.
	fmt.Fprintln(r.w, "### Descriptive Statistics")
	fmt.Fprintln(r.w)
	fmt.Fprintln(r.w, "| Metric | "+comp.Strategy1+" | "+comp.Strategy2+" |")
	fmt.Fprintln(r.w, "|--------|"+strings.Repeat("-", len(comp.Strategy1)+2)+"|"+strings.Repeat("-", len(comp.Strategy2)+2)+"|")
	fmt.Fprintf(r.w, "| Mean | %.2f | %.2f |\n", comp.Stats1.Mean, comp.Stats2.Mean)
	fmt.Fprintf(r.w, "| Median | %.2f | %.2f |\n", comp.Stats1.Median, comp.Stats2.Median)
	fmt.Fprintf(r.w, "| Std Dev | %.2f | %.2f |\n", comp.Stats1.StdDev, comp.Stats2.StdDev)
	fmt.Fprintf(r.w, "| Min | %.0f | %.0f |\n", comp.Stats1.Min, comp.Stats2.Min)
	fmt.Fprintf(r.w, "| Max | %.0f | %.0f |\n", comp.Stats1.Max, comp.Stats2.Max)
	fmt.Fprintln(r.w)

	// Statistical tests.
	fmt.Fprintln(r.w, "### Statistical Analysis")
	fmt.Fprintln(r.w)
	fmt.Fprintf(r.w, "- **Mann-Whitney U:** %.2f (z=%.2f, p=%.4f)\n",
		comp.MannWhitney.U, comp.MannWhitney.Z, comp.MannWhitney.PValue)
	fmt.Fprintf(r.w, "- **Effect size (Cohen's d):** %.2f (%s)\n",
		comp.EffectSize.CohensD, comp.EffectSize.Interpretation)
	fmt.Fprintf(r.w, "- **95%% CI for mean difference:** [%.2f, %.2f]\n",
		comp.BootstrapCI.LowerBound, comp.BootstrapCI.UpperBound)
	fmt.Fprintln(r.w)

	// Conclusion.
	fmt.Fprintln(r.w, "### Conclusion")
	fmt.Fprintln(r.w)
	if comp.WinnerConfident {
		fmt.Fprintf(r.w, "**%s** shows statistically significant improvement over %s ",
			comp.Winner, otherStrategy(comp.Winner, comp.Strategy1, comp.Strategy2))
		fmt.Fprintf(r.w, "(p < 0.05, effect size: %s).\n", comp.EffectSize.Interpretation)
	} else {
		fmt.Fprintln(r.w, "No statistically significant difference detected between strategies (p >= 0.05).")
	}
	fmt.Fprintln(r.w)
}

func otherStrategy(winner, s1, s2 string) string {
	if winner == s1 {
		return s2
	}
	return s1
}

// WriteDistributionChart writes an ASCII distribution chart.
func (r *MarkdownReport) WriteDistributionChart(name string, data []int) {
	fmt.Fprintf(r.w, "### %s Distribution\n\n", name)
	fmt.Fprintln(r.w, "```")

	// Create histogram.
	hist := makeHistogram(data, 10)
	maxCount := 0
	for _, count := range hist {
		if count > maxCount {
			maxCount = count
		}
	}

	// Print histogram.
	width := 40
	for i, count := range hist {
		barLen := 0
		if maxCount > 0 {
			barLen = count * width / maxCount
		}
		bar := strings.Repeat("█", barLen)
		fmt.Fprintf(r.w, "%3d-%3d │ %s %d\n", i*10, (i+1)*10-1, bar, count)
	}

	fmt.Fprintln(r.w, "```")
	fmt.Fprintln(r.w)
}

func makeHistogram(data []int, buckets int) []int {
	if len(data) == 0 {
		return make([]int, buckets)
	}

	min, max := data[0], data[0]
	for _, v := range data {
		if v < min {
			min = v
		}
		if v > max {
			max = v
		}
	}

	if max == min {
		max = min + 1
	}

	hist := make([]int, buckets)
	bucketSize := float64(max-min+1) / float64(buckets)

	for _, v := range data {
		bucket := int(float64(v-min) / bucketSize)
		if bucket >= buckets {
			bucket = buckets - 1
		}
		hist[bucket]++
	}

	return hist
}

// WriteFooter writes the report footer.
func (r *MarkdownReport) WriteFooter() {
	fmt.Fprintln(r.w, "---")
	fmt.Fprintln(r.w)
	fmt.Fprintln(r.w, "*Report generated by stockpile-bench*")
}
