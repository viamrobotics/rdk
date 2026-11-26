package armplanning

import (
	"fmt"
	"maps"
	"slices"

	"go.viam.com/rdk/referenceframe"
)

// freezeFrames returns a clone of fs in which each named frame is a 0-DoF static frame at its pose
// for its inputs in start, carrying its geometries as placed at those inputs under their original
// labels, so they stay collision obstacles and collision allows naming the frame or its geometries
// still match. The frozen inputs are returned keyed by frame name.
func freezeFrames(
	fs *referenceframe.FrameSystem, names []string, start referenceframe.FrameSystemInputs,
) (*referenceframe.FrameSystem, referenceframe.FrameSystemInputs, error) {
	if len(names) == 0 {
		return fs, nil, nil
	}
	frozenFS, err := fs.Clone()
	if err != nil {
		return nil, nil, err
	}
	frozen := referenceframe.FrameSystemInputs{}
	for _, name := range names {
		if _, done := frozen[name]; done {
			continue
		}
		f := frozenFS.Frame(name)
		if f == nil {
			return nil, nil, fmt.Errorf("cannot freeze frame %q: %w", name, referenceframe.NewFrameMissingError(name))
		}
		if len(f.DoF()) == 0 {
			return nil, nil, fmt.Errorf("cannot freeze frame %q: it has no degrees of freedom", name)
		}
		inputs, ok := start[name]
		if !ok {
			return nil, nil, fmt.Errorf("cannot freeze frame %q: start state has no configuration for it", name)
		}
		pose, err := f.Transform(inputs)
		if err != nil {
			return nil, nil, fmt.Errorf("cannot freeze frame %q: %w", name, err)
		}
		gif, err := f.Geometries(inputs)
		if err != nil {
			return nil, nil, fmt.Errorf("cannot freeze frame %q: %w", name, err)
		}
		// Every frame type expresses its geometries in its parent frame, so they carry over unchanged.
		static, err := referenceframe.NewStaticFrameWithGeometries(name, pose, gif.Geometries())
		if err != nil {
			return nil, nil, err
		}
		if err := frozenFS.ReplaceFrame(static); err != nil {
			return nil, nil, err
		}
		frozen[name] = inputs
	}
	return frozenFS, frozen, nil
}

// frozenRequest returns a copy of req that plans against a frame system with the frames named by
// req.Constraints.FrozenFrameConstraint frozen at their start inputs, and those inputs. req itself is
// left untouched: PlanMotionSequence reuses requests and keys backtracking on their *PlanState pointers.
//
// In the copy:
//   - start and goal configurations omit frozen frames. A goal value for a frozen frame must be within
//     PlannerOptions.InputIdentDist of its start value; a goal naming only frozen frames becomes a
//     no-op goal at the start configuration.
//   - rejectedSolutions is re-keyed to the copy's StartState and stripped of frozen frames, since the
//     plan manager looks rejections up by pointer and compares only the frames each one names.
//   - frozen frames are dropped from the cable constraint, which needs kinematic models. When that
//     empties CableConstraintFrames the constraint is turned off, as an empty list means every model.
func frozenRequest(req *PlanRequest) (*PlanRequest, referenceframe.FrameSystemInputs, error) {
	if req.Constraints == nil || len(req.Constraints.FrozenFrameConstraint) == 0 {
		return req, nil, nil
	}
	names := make([]string, 0, len(req.Constraints.FrozenFrameConstraint))
	for _, c := range req.Constraints.FrozenFrameConstraint {
		names = append(names, c.Frame)
	}
	frozenFS, frozen, err := freezeFrames(req.FrameSystem, names, req.StartState.Configuration())
	if err != nil {
		return nil, nil, err
	}
	startConf := withoutInputs(req.StartState.Configuration(), frozen)

	goals := make([]*PlanState, len(req.Goals))
	for i, goal := range req.Goals {
		conf := goal.Configuration()
		for name, want := range frozen {
			got, ok := conf[name]
			if ok && referenceframe.InputsLinfDistance(got, want) > req.PlannerOptions.InputIdentDist {
				return nil, nil, fmt.Errorf("goal %d sets configuration %v for frame %q, which is frozen at %v", i, got, name, want)
			}
		}
		goalConf := withoutInputs(conf, frozen)
		if len(conf) > 0 && len(goalConf) == 0 && len(goal.Poses()) == 0 {
			goalConf = startConf
		}
		goals[i] = NewPlanState(goal.Poses(), goalConf)
	}

	planReq := *req
	planReq.FrameSystem = frozenFS
	planReq.StartState = NewPlanState(req.StartState.Poses(), startConf)
	planReq.Goals = goals
	planReq.PlannerOptions = withoutFrozenCableFrames(req.PlannerOptions, frozen)
	if rejected, ok := req.rejectedSolutions[req.StartState]; ok {
		stripped := make([]referenceframe.FrameSystemInputs, len(rejected))
		for i, sol := range rejected {
			stripped[i] = withoutInputs(sol, frozen)
		}
		planReq.rejectedSolutions = map[*PlanState][]referenceframe.FrameSystemInputs{planReq.StartState: stripped}
	}
	return &planReq, frozen, nil
}

func withoutFrozenCableFrames(opts *PlannerOptions, frozen referenceframe.FrameSystemInputs) *PlannerOptions {
	if !opts.CableConstraint {
		return opts
	}
	out := *opts
	out.CableTwistReferenceDegs = maps.Clone(opts.CableTwistReferenceDegs)
	maps.DeleteFunc(out.CableTwistReferenceDegs, func(name string, _ []float64) bool { _, ok := frozen[name]; return ok })
	if len(opts.CableConstraintFrames) > 0 {
		out.CableConstraintFrames = slices.DeleteFunc(slices.Clone(opts.CableConstraintFrames),
			func(name string) bool { _, ok := frozen[name]; return ok })
		out.CableConstraint = len(out.CableConstraintFrames) > 0
	}
	return &out
}

func withoutInputs(conf, drop referenceframe.FrameSystemInputs) referenceframe.FrameSystemInputs {
	if conf == nil {
		return nil
	}
	out := make(referenceframe.FrameSystemInputs, len(conf))
	for name, inputs := range conf {
		if _, ok := drop[name]; !ok {
			out[name] = inputs
		}
	}
	return out
}

// withFrozenInputs returns traj with the frozen inputs added to every step.
func withFrozenInputs(traj []*referenceframe.LinearInputs, frozen referenceframe.FrameSystemInputs) []*referenceframe.LinearInputs {
	if len(frozen) == 0 {
		return traj
	}
	out := make([]*referenceframe.LinearInputs, len(traj))
	for i, step := range traj {
		conf := step.ToFrameSystemInputs()
		maps.Copy(conf, frozen)
		out[i] = conf.ToLinearInputs()
	}
	return out
}
