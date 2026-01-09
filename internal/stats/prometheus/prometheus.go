// Package prometheus provides a Prometheus-based stats collector.
package prometheus

import (
	"sync"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/discochess/stockpile/internal/stats"
)

// Collector implements stats.Collector using Prometheus metrics.
type Collector struct {
	registry prometheus.Registerer

	mu         sync.RWMutex
	counters   map[string]prometheus.Counter
	gauges     map[string]prometheus.Gauge
	histograms map[string]prometheus.Histogram
}

// Compile-time check that Collector implements stats.Collector.
var _ stats.Collector = (*Collector)(nil)

// New creates a new Prometheus collector.
// If registry is nil, prometheus.DefaultRegisterer is used.
func New(registry prometheus.Registerer) *Collector {
	if registry == nil {
		registry = prometheus.DefaultRegisterer
	}
	return &Collector{
		registry:   registry,
		counters:   make(map[string]prometheus.Counter),
		gauges:     make(map[string]prometheus.Gauge),
		histograms: make(map[string]prometheus.Histogram),
	}
}

// IncCounter increments a counter metric.
func (c *Collector) IncCounter(name string, delta int64) {
	counter := c.getOrCreateCounter(name)
	counter.Add(float64(delta))
}

// SetGauge sets a gauge metric.
func (c *Collector) SetGauge(name string, value int64) {
	gauge := c.getOrCreateGauge(name)
	gauge.Set(float64(value))
}

// ObserveHistogram records a value in a histogram.
func (c *Collector) ObserveHistogram(name string, value float64) {
	histogram := c.getOrCreateHistogram(name)
	histogram.Observe(value)
}

func (c *Collector) getOrCreateCounter(name string) prometheus.Counter {
	c.mu.RLock()
	counter, ok := c.counters[name]
	c.mu.RUnlock()
	if ok {
		return counter
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	// Double-check after acquiring write lock.
	if counter, ok = c.counters[name]; ok {
		return counter
	}

	counter = prometheus.NewCounter(prometheus.CounterOpts{
		Name: name,
		Help: name,
	})
	if err := c.registry.Register(counter); err != nil {
		// If already registered, try to get the existing metric.
		if are, ok := err.(prometheus.AlreadyRegisteredError); ok {
			if existing, ok := are.ExistingCollector.(prometheus.Counter); ok {
				c.counters[name] = existing
				return existing
			}
		}
		// Fallback: return the new counter anyway (registration failed but metric works).
	}
	c.counters[name] = counter
	return counter
}

func (c *Collector) getOrCreateGauge(name string) prometheus.Gauge {
	c.mu.RLock()
	gauge, ok := c.gauges[name]
	c.mu.RUnlock()
	if ok {
		return gauge
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if gauge, ok = c.gauges[name]; ok {
		return gauge
	}

	gauge = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: name,
		Help: name,
	})
	if err := c.registry.Register(gauge); err != nil {
		if are, ok := err.(prometheus.AlreadyRegisteredError); ok {
			if existing, ok := are.ExistingCollector.(prometheus.Gauge); ok {
				c.gauges[name] = existing
				return existing
			}
		}
	}
	c.gauges[name] = gauge
	return gauge
}

func (c *Collector) getOrCreateHistogram(name string) prometheus.Histogram {
	c.mu.RLock()
	histogram, ok := c.histograms[name]
	c.mu.RUnlock()
	if ok {
		return histogram
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if histogram, ok = c.histograms[name]; ok {
		return histogram
	}

	histogram = prometheus.NewHistogram(prometheus.HistogramOpts{
		Name:    name,
		Help:    name,
		Buckets: prometheus.DefBuckets,
	})
	if err := c.registry.Register(histogram); err != nil {
		if are, ok := err.(prometheus.AlreadyRegisteredError); ok {
			if existing, ok := are.ExistingCollector.(prometheus.Histogram); ok {
				c.histograms[name] = existing
				return existing
			}
		}
	}
	c.histograms[name] = histogram
	return histogram
}
