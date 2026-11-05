package armplanning

import (
	"context"
	"fmt"
	"math"
	"strings"

	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/motionplan/ik"
	"go.viam.com/rdk/referenceframe"
)

// pinnedTolerance is how close, in a joint's native units, the IK's closest configuration must be
// to a limit for that limit to count as the one stopping it.
const pinnedTolerance = 1e-4

// jointLimitError reports a goal that the IK could reach if some joints were allowed past their
// limits. It wraps errIKSolve so callers matching on that keep working.
type jointLimitError struct {
	blocked []blockedJoint
}

type blockedJoint struct {
	frame, joint     string
	needed, min, max float64
	units            string
}

func (e *jointLimitError) Error() string {
	parts := make([]string, 0, len(e.blocked))
	for _, b := range e.blocked {
		parts = append(parts, fmt.Sprintf("%q joint %q would need %.1f%s but its limits are [%.1f%s, %.1f%s]",
			b.frame, b.joint, b.needed, b.units, b.min, b.units, b.max, b.units))
	}
	return "zero IK solutions produced, the goal is reachable only past joint limits: " + strings.Join(parts, "; ")
}

func (e *jointLimitError) Unwrap() error { return errIKSolve }

// diagnoseUnreachable explains a zero-solution IK. It takes the closest configuration the solver
// reached, and if some joints there sit on a limit, re-runs one descent from it with those limits
// opened up. Converging then means those limits, not reach or the goal itself, are what make the
// goal unsatisfiable. Anything else returns errIKSolve unchanged.
func diagnoseUnreachable(
	ctx context.Context,
	psc *PlanSegmentContext,
	minFunc ik.CostFunc,
	metas []ik.SeedSolveMetaData,
	logger logging.Logger,
) error {
	var closest ik.SeedSolveMetaData
	for i := range metas {
		if metas[i].BestConfiguration != nil &&
			(closest.BestConfiguration == nil || metas[i].BestScore < closest.BestScore) {
			closest = metas[i]
		}
	}
	if closest.BestConfiguration == nil {
		return errIKSolve
	}

	limits := psc.pc.lis.GetLimits()
	if len(limits) != len(closest.BestConfiguration) {
		return errIKSolve
	}

	relaxed := make([]referenceframe.Limit, len(limits))
	copy(relaxed, limits)
	pinned := []int{}
	for i, l := range limits {
		v := closest.BestConfiguration[i]
		span := l.Max - l.Min
		switch {
		case v-l.Min <= pinnedTolerance:
			relaxed[i].Min -= span
		case l.Max-v <= pinnedTolerance:
			relaxed[i].Max += span
		default:
			continue
		}
		pinned = append(pinned, i)
	}
	if len(pinned) == 0 {
		return errIKSolve
	}

	solver, err := ik.CreateNloptSolver(logger, 1, true, true, 0)
	if err != nil {
		return errIKSolve
	}
	solution, err := solver.SolveOnce(ctx, minFunc, closest.BestConfiguration, relaxed)
	if err != nil {
		logger.Debugf("joints %v sit on a limit at the closest IK configuration, but relaxing them does not reach the goal: %v",
			pinned, err)
		return errIKSolve
	}

	blocked := []blockedJoint{}
	for _, i := range pinned {
		if solution[i] < limits[i].Min || solution[i] > limits[i].Max {
			blocked = append(blocked, describeJoint(psc.pc.lis, i, solution[i], limits[i]))
		}
	}
	if len(blocked) == 0 {
		return errIKSolve
	}
	return &jointLimitError{blocked: blocked}
}

// describeJoint names the joint behind linearized input idx and expresses its values in the units
// the frame reports joint positions in (degrees for revolute joints of a model).
func describeJoint(lis *referenceframe.LinearInputsSchema, idx int, needed float64, limit referenceframe.Limit) blockedJoint {
	frame, dofIdx := lis.FrameForIndex(idx)
	b := blockedJoint{
		joint:  fmt.Sprintf("%d", dofIdx),
		needed: needed,
		min:    limit.Min,
		max:    limit.Max,
	}
	if frame == nil {
		return b
	}
	b.frame = frame.Name()

	if model, ok := frame.(*referenceframe.SimpleModel); ok {
		if names := model.MoveableFrameNames(); len(names) == len(frame.DoF()) {
			b.joint = names[dofIdx]
		}
	}

	// ProtobufFromInput is the frame's own conversion to user-facing units: degrees for a revolute
	// joint, mm for a prismatic one.
	toUnits := func(v float64) (float64, bool) {
		inputs := make([]referenceframe.Input, len(frame.DoF()))
		inputs[dofIdx] = v
		jp := frame.ProtobufFromInput(inputs)
		if jp == nil || len(jp.Values) != len(inputs) {
			return 0, false
		}
		return jp.Values[dofIdx], true
	}
	needUnits, ok1 := toUnits(needed)
	minUnits, ok2 := toUnits(limit.Min)
	maxUnits, ok3 := toUnits(limit.Max)
	unit, ok4 := toUnits(1)
	if ok1 && ok2 && ok3 && ok4 {
		b.needed, b.min, b.max = needUnits, minUnits, maxUnits
		if math.Abs(unit-180/math.Pi) < 1e-9 {
			b.units = "°"
		}
	}
	return b
}
