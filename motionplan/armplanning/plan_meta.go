package armplanning

import (
	"fmt"
	"io"
	"sync"
	"sync/atomic"
	"time"
)

// InvocationCounters is used to count the number of times a method has been invoked and the
// accumulated time spent in that function.
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

// NewPlanMeta constructs PlanMeta.
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

// AddTiming will increment the invocation count and time spent for an "operation".
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

// OutputTiming pretty-prints in a text format the timing information for a motion plan.
func (pm *PlanMeta) OutputTiming(outputWriter io.Writer) {
	if _, exists := pm.Timing["planToDirectJoints"]; exists {
		// Joint -> Joint(s) request
		//nolint:errcheck
		fmt.Fprintf(outputWriter, `PlanMotion:						%v
  newPlanContext:				%v
  planMultiWaypoint:			%v
    ComputePoses:				%v
    planToDirectJoints:			%v
      newPlanSegmentContext:	%v
      newCBiRRTMotionPlanner:	%v
      rrtRunner:				%v
        constrainedExtend:		%v
      smoothPathSimple:			%v
        simpleSmoothStep:		%v
          checkPath:			%v
`,
			pm.Timing["PlanMotion"],
			pm.Timing["newPlanContext"],
			pm.Timing["planMultiWaypoint"],
			pm.Timing["ComputePoses"],
			pm.Timing["planToDirectJoints"],
			pm.Timing["newPlanSegmentContext"],
			pm.Timing["newCBiRRTMotionPlanner"],
			pm.Timing["rrtRunner"],
			pm.Timing["constrainedExtend"],
			pm.Timing["smoothPathSimple"],
			pm.Timing["simpleSmoothStep"],
			pm.Timing["checkPath"],
		)
	} else {
		// Joint -> Pose(s) request
		//nolint:errcheck
		fmt.Fprintf(outputWriter, `PlanMotion:						%v
  newPlanContext:				%v
  planMultiWaypoint:			%v
    ComputePoses:				%v
    generateWaypoints:			%v
    planSingleGoal:				%v
      newPlanSegmentContext:	%v
      initRRTSolutions:			%v
        sss.process:			%v
      newCBiRRTMotionPlanner:	%v
      rrtRunner:				%v
        constrainedExtend:		%v
      smoothPathSimple:			%v
        simpleSmoothStep:		%v
  checkPath (smooth + process):	%v
`,
			pm.Timing["PlanMotion"],
			pm.Timing["newPlanContext"],
			pm.Timing["planMultiWaypoint"],
			pm.Timing["ComputePoses"],
			pm.Timing["generateWaypoints"],
			pm.Timing["planSingleGoal"],
			pm.Timing["newPlanSegmentContext"],
			pm.Timing["initRRTSolutions"],
			pm.Timing["sss.process"],
			pm.Timing["newCBiRRTMotionPlanner"],
			pm.Timing["rrtRunner"],
			pm.Timing["constrainedExtend"],
			pm.Timing["smoothPathSimple"],
			pm.Timing["simpleSmoothStep"],
			pm.Timing["checkPath"],
		)
	}
}

// Calls returns the number of times a function was called.
func (ic *InvocationCounters) Calls() int64 {
	if ic == nil {
		return 0
	}

	return ic.calls.Load()
}

// TotalTimeNanos returns the total accumulated runtime of a function as a time in nanoseconds.
func (ic *InvocationCounters) TotalTimeNanos() int64 {
	if ic == nil {
		return 0
	}

	return ic.timeNanos.Load()
}

// TotalTime returns the total accumulated runtime of a function as a time.Duration.
func (ic *InvocationCounters) TotalTime() time.Duration {
	return time.Duration(ic.TotalTimeNanos())
}

// Average returns the average time spent per function invocation. Returns a zero-value when a
// function was not called.
func (ic *InvocationCounters) Average() time.Duration {
	calls := ic.Calls()
	if calls == 0 {
		return time.Duration(0)
	}

	return time.Duration(ic.timeNanos.Load() / calls)
}

// String is a pretty-formated string representation of the number of calls/total time/average.
func (ic *InvocationCounters) String() string {
	// Calls is fixed at three spaces, right aligned.
	// Total time is fixed at thirteen spaces, left aligned.
	return fmt.Sprintf("Calls: %3d Total time: %-13s Average time: %v",
		ic.Calls(), ic.TotalTime(), ic.Average())
}
