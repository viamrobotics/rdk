// Package internal holds the global state for Statz
package internal

import (
	"fmt"
	"runtime"
	"sync"

	"github.com/edaniels/golog"
)

type global struct {
	mu sync.Mutex
	// Register the metric name to the callers file#line location.
	metrics map[string]string
}

var state global

// RegisterMetric validates and registers a statz metric. Must be unique within the application.
// Panic on any failures to ensure we catch the errors early instead of loosing metrics.
func RegisterMetric(name string) {
	state.mu.Lock()
	defer state.mu.Unlock()

	if state.metrics == nil {
		state.metrics = make(map[string]string)
	}

	var caller string

	// Try to help users who define metrics twice by printing what registered the metric.
	_, file, no, ok := runtime.Caller(4)
	if ok {
		caller = fmt.Sprintf("%s#%d", file, no)
	}

	if prev, ok := state.metrics[name]; ok {
		golog.Global().Panicf(`Metric %s was already defined and is trying to register again. It may be registered at: %s
			"Statz metrics MUST be globalally unique in the application.`, name, prev)
		return
	}

	state.metrics[name] = caller
}
