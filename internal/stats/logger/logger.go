// Package logger provides a zap-based stats collector that logs metrics.
package logger

import (
	"go.uber.org/zap"

	"github.com/discochess/stockpile/internal/stats"
)

// Collector implements stats.Collector by logging metrics via zap.
type Collector struct {
	logger *zap.Logger
}

// Compile-time check that Collector implements stats.Collector.
var _ stats.Collector = (*Collector)(nil)

// New creates a new logger-based collector.
// If logger is nil, a no-op logger is used.
func New(logger *zap.Logger) *Collector {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &Collector{logger: logger}
}

// IncCounter logs a counter increment.
func (c *Collector) IncCounter(name string, delta int64) {
	c.logger.Debug("counter",
		zap.String("metric", name),
		zap.Int64("delta", delta),
	)
}

// SetGauge logs a gauge value.
func (c *Collector) SetGauge(name string, value int64) {
	c.logger.Debug("gauge",
		zap.String("metric", name),
		zap.Int64("value", value),
	)
}

// ObserveHistogram logs a histogram observation.
func (c *Collector) ObserveHistogram(name string, value float64) {
	c.logger.Debug("histogram",
		zap.String("metric", name),
		zap.Float64("value", value),
	)
}
