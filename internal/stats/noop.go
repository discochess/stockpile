package stats

// Noop is a no-op collector that discards all metrics.
// Useful for testing or when metrics are not needed.
type Noop struct{}

// Compile-time check that Noop implements Collector.
var _ Collector = (*Noop)(nil)

// NewNoop creates a new no-op collector.
func NewNoop() *Noop {
	return &Noop{}
}

func (n *Noop) IncCounter(name string, delta int64)       {}
func (n *Noop) SetGauge(name string, value int64)         {}
func (n *Noop) ObserveHistogram(name string, value float64) {}
