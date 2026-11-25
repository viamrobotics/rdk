package armplanning

import (
	"fmt"
	"maps"
	"slices"

	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/spatialmath"
)

// freezeFrames returns a clone of fs in which each named frame is a 0-DoF static frame at its pose
// for its inputs in start. Its geometries, placed at those inputs, move to a static sibling frame
// named <name>_frozen_geometry so they stay collision obstacles under their original labels. The
// frozen inputs are returned keyed by frame name.
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
		// Geometries come back expressed in the parent frame, so they hang off the parent at a zero
		// pose rather than off the frozen frame, which would apply pose twice.
		gif, err := f.Geometries(inputs)
		if err != nil {
			return nil, nil, fmt.Errorf("cannot freeze frame %q: %w", name, err)
		}
		parent, err := frozenFS.Parent(f)
		if err != nil {
			return nil, nil, err
		}
		static, err := referenceframe.NewStaticFrame(name, pose)
		if err != nil {
			return nil, nil, err
		}
		if err := frozenFS.ReplaceFrame(static); err != nil {
			return nil, nil, err
		}
		if geoms := gif.Geometries(); len(geoms) > 0 {
			geomFrame, err := referenceframe.NewStaticFrameWithGeometries(name+"_frozen_geometry", spatialmath.NewZeroPose(), geoms)
			if err != nil {
				return nil, nil, err
			}
			if err := frozenFS.AddFrame(geomFrame, frozenFS.Frame(parent.Name())); err != nil {
				return nil, nil, err
			}
		}
		frozen[name] = inputs
	}
	return frozenFS, frozen, nil
}

// frozenRequest returns a copy of req that plans against a frame system with
// req.Constraints.FrozenFrames frozen at their start inputs, and those inputs. The copy's start
// and goal configurations omit the frozen frames. req itself is left untouched: PlanMotionSequence
// reuses requests and keys backtracking on their *PlanState pointers.
func frozenRequest(req *PlanRequest) (*PlanRequest, referenceframe.FrameSystemInputs, error) {
	if req.Constraints == nil || len(req.Constraints.FrozenFrames) == 0 {
		return req, nil, nil
	}
	frozenFS, frozen, err := freezeFrames(req.FrameSystem, req.Constraints.FrozenFrames, req.StartState.Configuration())
	if err != nil {
		return nil, nil, err
	}

	goals := make([]*PlanState, len(req.Goals))
	for i, goal := range req.Goals {
		conf := goal.Configuration()
		for name, want := range frozen {
			got, ok := conf[name]
			if ok && !slices.Equal(got, want) {
				return nil, nil, fmt.Errorf("goal %d sets configuration %v for frame %q, which is frozen at %v", i, got, name, want)
			}
		}
		goals[i] = NewPlanState(goal.Poses(), withoutInputs(conf, frozen))
	}

	planReq := *req
	planReq.FrameSystem = frozenFS
	planReq.StartState = NewPlanState(req.StartState.Poses(), withoutInputs(req.StartState.Configuration(), frozen))
	planReq.Goals = goals
	// The plan manager looks rejections up by StartState pointer and compares only the frames
	// present in each rejected configuration, so both the key and the contents must match the copy.
	if rejected, ok := req.rejectedSolutions[req.StartState]; ok {
		stripped := make([]referenceframe.FrameSystemInputs, len(rejected))
		for i, sol := range rejected {
			stripped[i] = withoutInputs(sol, frozen)
		}
		planReq.rejectedSolutions = map[*PlanState][]referenceframe.FrameSystemInputs{planReq.StartState: stripped}
	}
	return &planReq, frozen, nil
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
