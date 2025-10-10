package armplanning

import (
	"fmt"
	"io"
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

func (pm *PlanMeta) OutputTiming(outputWriter io.Writer) {
	if _, exists := pm.Timing["planToDirectJoints"]; exists {
		// Joint -> Joint(s) request
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
		fmt.Fprintf(outputWriter, `PlanMotion:						%v
  newPlanContext:				%v
  planMultiWaypoint:			%v
    ComputePoses:				%v
    generateWaypoints:			%v
    planSingleGoal:				%v
      newPlanSegmentContext:	%v
      initRRTSolutions:			%v
      newCBiRRTMotionPlanner:	%v
      rrtRunner:				%v
        constrainedExtend:		%v
      smoothPathSimple:			%v
        simpleSmoothStep:		%v
        checkPath:				%v
`,
			pm.Timing["PlanMotion"],
			pm.Timing["newPlanContext"],
			pm.Timing["planMultiWaypoint"],
			pm.Timing["ComputePoses"],
			pm.Timing["generateWaypoints"],
			pm.Timing["planSingleGoal"],
			pm.Timing["newPlanSegmentContext"],
			pm.Timing["initRRTSolutions"],
			pm.Timing["newCBiRRTMotionPlanner"],
			pm.Timing["rrtRunner"],
			pm.Timing["constrainedExtend"],
			pm.Timing["smoothPathSimple"],
			pm.Timing["simpleSmoothStep"],
			pm.Timing["checkPath"],
		)
	}
}

func (ic *InvocationCounters) Calls() int64 {
	if ic == nil {
		return 0
	}

	return ic.calls.Load()
}

func (ic *InvocationCounters) TotalTimeNanos() int64 {
	if ic == nil {
		return 0
	}

	return ic.timeNanos.Load()
}

func (ic *InvocationCounters) TotalTime() time.Duration {
	return time.Duration(ic.TotalTimeNanos())
}

func (ic *InvocationCounters) Average() time.Duration {
	calls := ic.Calls()
	if calls == 0 {
		return time.Duration(0)
	}

	return time.Duration(ic.timeNanos.Load() / calls)
}

func (ic *InvocationCounters) String() string {
	// Calls is fixed at three spaces, right aligned.
	// Total time is fixed at thirteen spaces, left aligned.
	return fmt.Sprintf("Calls: %3d Total time: %-13s Average time: %v",
		ic.Calls(), ic.TotalTime(), ic.Average())
}
