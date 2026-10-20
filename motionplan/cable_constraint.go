package motionplan

import (
	"errors"
	"fmt"
	"math"

	pb "go.viam.com/api/component/arm/v1"

	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/utils"
)

// ErrCableConstraintViolated tags every cable-twist failure so callers can tell
// it apart from other constraint violations with errors.Is.
var ErrCableConstraintViolated = errors.New("cable constraint violated")

// DefaultMaxCableTwistDegs is the per-joint twist budget used when none is
// given. Every 360-degree-periodic pose has an equivalent within half a turn of
// the reference, so this rules out winding a cable the long way around without
// making any pose unreachable.
const DefaultMaxCableTwistDegs = 180.

// CableConstraint keeps cables routed through an arm from winding up: every
// rotational joint of every kinematic model (referenceframe.Model) in the frame
// system must stay within MaxTwistDegs of its reference position.
// The limit is per joint rather than a sum because a dressed cable fails where
// it is clamped or looped at an individual joint, and a box in joint space is
// convex, so a straight joint-space segment between two valid states is valid.
type CableConstraint struct {
	// MaxTwistDegs is how far any rotational joint may rotate away from its
	// reference. Defaults to DefaultMaxCableTwistDegs when <= 0.
	MaxTwistDegs float64

	// ReferenceDegs maps a frame name to the joint positions, in the frame's
	// protobuf units (degrees for rotational joints), at which its cables are
	// untwisted. Frames that are absent use the midpoint of each joint's limits,
	// so a joint whose range is not centered on zero keeps its full reach.
	ReferenceDegs map[string][]float64

	// Frames limits the constraint to the named kinematic models. Empty means
	// every model in the frame system.
	Frames []string
}

// cableJointBand is the allowed range of one rotational joint of a kinematic model.
type cableJointBand struct {
	frame string

	// idx is the joint's position in the frame's own DoF, so it indexes the slice returned by
	// Configuration.Get(frame). It is neither an offset into the flattened LinearInputs nor a count
	// of rotational joints: translational joints keep their slot, so the idx values of a frame's
	// bands can skip numbers.
	idx int

	min, max float64 // input units (radians)
}

// newCableConstraintCheck resolves the constraint against a frame system. A
// joint that already starts beyond its band has the band stretched to cover
// the start, so a plan may hold or unwind that twist but never add to it.
func newCableConstraintCheck(
	fs *referenceframe.FrameSystem,
	start *referenceframe.LinearInputs,
	cc CableConstraint,
) (StateFSConstraint, error) {
	maxTwist := cc.MaxTwistDegs
	if maxTwist <= 0 {
		maxTwist = DefaultMaxCableTwistDegs
	}
	maxTwistRads := utils.DegToRad(maxTwist)

	for name := range cc.ReferenceDegs {
		if _, ok := fs.Frame(name).(referenceframe.Model); !ok {
			return nil, fmt.Errorf("cable constraint reference frame %q is not a kinematic model in the frame system", name)
		}
	}

	names := cc.Frames
	if len(names) == 0 {
		names = fs.FrameNames()
	}
	for _, name := range cc.Frames {
		if _, ok := fs.Frame(name).(referenceframe.Model); !ok {
			return nil, fmt.Errorf("cable constraint frame %q is not a kinematic model in the frame system", name)
		}
	}

	var bands []cableJointBand
	for _, name := range names {
		frame, ok := fs.Frame(name).(referenceframe.Model)
		if !ok {
			// When `names = fs.FrameNames()`, we need to skip over non-model frames as the above
			// check did not cover these frame names.
			continue
		}
		limits := frame.DoF()
		dof := len(limits)
		if dof == 0 {
			continue
		}
		rotational := rotationalDoF(frame)

		ref := make([]referenceframe.Input, dof)
		for i, lim := range limits {
			ref[i] = (lim.Min + lim.Max) / 2
		}
		if refDegs, ok := cc.ReferenceDegs[name]; ok {
			if len(refDegs) != dof {
				return nil, fmt.Errorf("cable constraint reference for frame %q has %d values, frame has %d DoF",
					name, len(refDegs), dof)
			}
			ref = frame.InputFromProtobuf(&pb.JointPositions{Values: refDegs})
		}
		startInputs := start.Get(name)

		for i := range dof {
			if !rotational[i] {
				continue
			}
			band := cableJointBand{frame: name, idx: i, min: ref[i] - maxTwistRads, max: ref[i] + maxTwistRads}
			// Always true under the planner: collision detection requires a complete start
			// state. This check only keeps direct callers from indexing past the end.
			if i < len(startInputs) {
				band.min = min(band.min, startInputs[i])
				band.max = max(band.max, startInputs[i])
			}
			bands = append(bands, band)
		}
	}

	if len(bands) == 0 {
		return nil, nil
	}

	return func(state *StateFS) error {
		for _, b := range bands {
			inputs := state.Configuration.Get(b.frame)
			if b.idx >= len(inputs) {
				// Programmer error: the frame's inputs here no longer have the same shape the band
				// was built from. Silently allow the constraint checking to continue.
				continue
			}

			if v := inputs[b.idx]; v < b.min || v > b.max {
				return fmt.Errorf("%s joint %d %w: %0.2f degs outside [%0.2f, %0.2f]",
					b.frame, b.idx, ErrCableConstraintViolated,
					utils.RadToDeg(v), utils.RadToDeg(b.min), utils.RadToDeg(b.max))
			}
		}
		return nil
	}, nil
}

// rotationalDoF reports which of a frame's DoF are rotational, relying on the
// Frame contract that protobuf units are degrees for rotational joints and mm
// for translational ones.
func rotationalDoF(frame referenceframe.Model) []bool {
	dof := len(frame.DoF())
	ones := make([]referenceframe.Input, dof)
	for i := range ones {
		ones[i] = 1
	}
	out := make([]bool, dof)
	pbVals := frame.ProtobufFromInput(ones).GetValues()
	for i := 0; i < dof && i < len(pbVals); i++ {
		out[i] = math.Abs(pbVals[i]-utils.RadToDeg(1)) < 1e-9
	}
	return out
}

// AddCableConstraint adds a cable-twist check to the topological constraints,
// evaluated ahead of them since it needs no forward kinematics. Call it only after the checker is
// fully constructed: setting up the topological constraints replaces topoConstraint outright, so
// a cable check added earlier would be silently dropped.
func (c *ConstraintChecker) AddCableConstraint(
	fs *referenceframe.FrameSystem,
	start *referenceframe.LinearInputs,
	cc CableConstraint,
) error {
	check, err := newCableConstraintCheck(fs, start, cc)
	if err != nil || check == nil {
		return err
	}
	prev := c.topoConstraint
	c.topoConstraint = func(state *StateFS) error {
		if err := check(state); err != nil {
			return err
		}
		if prev != nil {
			return prev(state)
		}
		return nil
	}
	return nil
}
