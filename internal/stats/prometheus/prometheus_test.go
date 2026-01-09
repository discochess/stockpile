package prometheus

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
)

func TestNew_DefaultRegistry(t *testing.T) {
	// Create with nil registry - should use default.
	c := New(nil)
	if c == nil {
		t.Fatal("New(nil) returned nil")
	}
	if c.registry == nil {
		t.Error("registry should not be nil")
	}
}

func TestNew_CustomRegistry(t *testing.T) {
	reg := prometheus.NewRegistry()
	c := New(reg)
	if c.registry != reg {
		t.Error("registry should be the custom registry")
	}
}

func TestCollector_IncCounter(t *testing.T) {
	reg := prometheus.NewRegistry()
	c := New(reg)

	// Increment counter.
	c.IncCounter("test_counter", 5)
	c.IncCounter("test_counter", 3)

	// Verify counter was created and incremented.
	metrics, err := reg.Gather()
	if err != nil {
		t.Fatalf("Gather() error = %v", err)
	}

	found := false
	for _, m := range metrics {
		if m.GetName() == "test_counter" {
			found = true
			if len(m.GetMetric()) == 0 {
				t.Error("counter has no metrics")
				break
			}
			val := m.GetMetric()[0].GetCounter().GetValue()
			if val != 8 {
				t.Errorf("counter value = %v, want 8", val)
			}
		}
	}

	if !found {
		t.Error("counter test_counter not found in registry")
	}
}

func TestCollector_SetGauge(t *testing.T) {
	reg := prometheus.NewRegistry()
	c := New(reg)

	// Set gauge.
	c.SetGauge("test_gauge", 42)

	metrics, err := reg.Gather()
	if err != nil {
		t.Fatalf("Gather() error = %v", err)
	}

	found := false
	for _, m := range metrics {
		if m.GetName() == "test_gauge" {
			found = true
			if len(m.GetMetric()) == 0 {
				t.Error("gauge has no metrics")
				break
			}
			val := m.GetMetric()[0].GetGauge().GetValue()
			if val != 42 {
				t.Errorf("gauge value = %v, want 42", val)
			}
		}
	}

	if !found {
		t.Error("gauge test_gauge not found in registry")
	}
}

func TestCollector_ObserveHistogram(t *testing.T) {
	reg := prometheus.NewRegistry()
	c := New(reg)

	// Observe histogram.
	c.ObserveHistogram("test_histogram", 0.5)
	c.ObserveHistogram("test_histogram", 1.5)
	c.ObserveHistogram("test_histogram", 2.5)

	metrics, err := reg.Gather()
	if err != nil {
		t.Fatalf("Gather() error = %v", err)
	}

	found := false
	for _, m := range metrics {
		if m.GetName() == "test_histogram" {
			found = true
			if len(m.GetMetric()) == 0 {
				t.Error("histogram has no metrics")
				break
			}
			count := m.GetMetric()[0].GetHistogram().GetSampleCount()
			if count != 3 {
				t.Errorf("histogram count = %v, want 3", count)
			}
		}
	}

	if !found {
		t.Error("histogram test_histogram not found in registry")
	}
}

func TestCollector_ReuseMetrics(t *testing.T) {
	reg := prometheus.NewRegistry()
	c := New(reg)

	// Create same counter multiple times.
	c.IncCounter("reuse_test", 1)
	c.IncCounter("reuse_test", 1)
	c.IncCounter("reuse_test", 1)

	// Should have only one counter registered.
	metrics, err := reg.Gather()
	if err != nil {
		t.Fatalf("Gather() error = %v", err)
	}

	count := 0
	for _, m := range metrics {
		if m.GetName() == "reuse_test" {
			count++
		}
	}

	if count != 1 {
		t.Errorf("expected 1 metric named reuse_test, got %d", count)
	}
}

func TestCollector_ConcurrentAccess(t *testing.T) {
	reg := prometheus.NewRegistry()
	c := New(reg)

	// Access metrics concurrently.
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func(id int) {
			for j := 0; j < 100; j++ {
				c.IncCounter("concurrent_counter", 1)
				c.SetGauge("concurrent_gauge", int64(j))
				c.ObserveHistogram("concurrent_histogram", float64(j))
			}
			done <- true
		}(i)
	}

	// Wait for all goroutines.
	for i := 0; i < 10; i++ {
		<-done
	}

	// Verify metrics were created.
	metrics, err := reg.Gather()
	if err != nil {
		t.Fatalf("Gather() error = %v", err)
	}

	foundCounter := false
	foundGauge := false
	foundHistogram := false
	for _, m := range metrics {
		switch m.GetName() {
		case "concurrent_counter":
			foundCounter = true
			val := m.GetMetric()[0].GetCounter().GetValue()
			if val != 1000 { // 10 goroutines * 100 increments
				t.Errorf("counter value = %v, want 1000", val)
			}
		case "concurrent_gauge":
			foundGauge = true
		case "concurrent_histogram":
			foundHistogram = true
			count := m.GetMetric()[0].GetHistogram().GetSampleCount()
			if count != 1000 {
				t.Errorf("histogram count = %v, want 1000", count)
			}
		}
	}

	if !foundCounter {
		t.Error("concurrent_counter not found")
	}
	if !foundGauge {
		t.Error("concurrent_gauge not found")
	}
	if !foundHistogram {
		t.Error("concurrent_histogram not found")
	}
}

func TestCollector_AlreadyRegistered(t *testing.T) {
	reg := prometheus.NewRegistry()

	// Pre-register a counter with the same name.
	existingCounter := prometheus.NewCounter(prometheus.CounterOpts{
		Name: "preexisting_counter",
		Help: "preexisting_counter",
	})
	reg.MustRegister(existingCounter)
	existingCounter.Add(100)

	// Create collector and try to use same metric name.
	c := New(reg)
	c.IncCounter("preexisting_counter", 5)

	// Should not panic and should reuse existing counter.
	metrics, err := reg.Gather()
	if err != nil {
		t.Fatalf("Gather() error = %v", err)
	}

	for _, m := range metrics {
		if m.GetName() == "preexisting_counter" {
			val := m.GetMetric()[0].GetCounter().GetValue()
			// Should be 105 (100 from original + 5 from collector).
			if val != 105 {
				t.Errorf("counter value = %v, want 105", val)
			}
		}
	}
}
