package armplanning

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

type InvocationCounters struct {
	calls     atomic.Int64
	timeNanos atomic.Int64
}

// PlanMeta is meta data about plan generation.
type PlanMeta struct {
	Duration       time.Duration
	Partial        bool
	GoalsProcessed int

	timingMu sync.Mutex
	Timing   map[string]*InvocationCounters
}

func NewPlanMeta() *PlanMeta {
	return &PlanMeta{
		Timing: make(map[string]*InvocationCounters),
	}
}

// DeferTiming can be used as a one-liner for tracking a function invocation. Expected usage at the
// top of a function is:
//
//	defer planMeta.DeferTiming("functionName", time.Now())
//
// Note this helper/usage is "clever" in that it the above example `time.Now()` is computed at the
// beginning of the function. But moving this call into a larger `defer func () { DeferTiming(...)
// }()` would break when the start time is computed.
func (pm *PlanMeta) DeferTiming(opName string, start time.Time) {
	pm.AddTiming(opName, time.Since(start))
}

func (pm *PlanMeta) AddTiming(opName string, dur time.Duration) {
	pm.timingMu.Lock()
	defer pm.timingMu.Unlock()

	if foo, exists := pm.Timing[opName]; exists {
		foo.calls.Add(1)
		foo.timeNanos.Add(dur.Nanoseconds())
	} else {
		foo := &InvocationCounters{}
		foo.calls.Store(1)
		foo.timeNanos.Store(dur.Nanoseconds())
		pm.Timing[opName] = foo
	}
}

func (ic *InvocationCounters) Average() time.Duration {
	calls := ic.calls.Load()
	if calls == 0 {
		return time.Duration(0)
	}

	return time.Duration(ic.timeNanos.Load() / calls)
}

func (foo *InvocationCounters) String() string {
	return fmt.Sprintf("Calls: %v Total time: %v Average time: %v",
		foo.calls.Load(), foo.timeNanos.Load())
}
