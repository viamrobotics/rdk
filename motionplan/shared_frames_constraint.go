package motionplan

import (
	"errors"
	"fmt"
	"math"

	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/utils"
)

// ErrSharedFramesConstraintViolated tags every shared-frames-disagreement
// failure so callers can tell it apart from other constraint violations with
// errors.Is.
var ErrSharedFramesConstraintViolated = errors.New("shared frames constraint violated")

// SharedFramesConstraint requires a set of frames to all share the same
// world pose within a tolerance. Primary use: two arms rigidly holding a
// single tool — both arms' grip frames evaluate to the same world pose as
// long as the tool isn't physically deforming.
//
// For planners that explore joint space (cBiRRT and friends), this
// constraint is what keeps RRT from wasting time on samples that place the
// two grips at different world positions. The goal metric alone can't do
// that: it rewards getting close to the goal, but says nothing about
// whether the shared-tool invariant holds mid-trajectory.
type SharedFramesConstraint struct {
	// Frames names the frames whose world poses must agree. Must contain at
	// least two entries. The check pins every frame to the first entry's
	// pose, so the order doesn't affect semantics — Frames[0] is just the
	// reference every other frame is compared against.
	Frames []string

	// PositionToleranceMM: how far frames may disagree in position (mm).
	PositionToleranceMM float64

	// OrientationToleranceDegs: how far frames may disagree in orientation.
	OrientationToleranceDegs float64
}

// DefaultSharedFramesPositionToleranceMM: matches CableConstraint's style of
// a gentle default suitable for a soft/compliant grip.
const DefaultSharedFramesPositionToleranceMM = 10.0

// DefaultSharedFramesOrientationToleranceDegs: ditto.
const DefaultSharedFramesOrientationToleranceDegs = 2.0

func newSharedFramesConstraintCheck(
	fs *referenceframe.FrameSystem,
	sfc SharedFramesConstraint,
) (StateFSConstraint, error) {
	if len(sfc.Frames) < 2 {
		return nil, fmt.Errorf("shared-frames constraint needs ≥ 2 frames, got %d", len(sfc.Frames))
	}
	for _, name := range sfc.Frames {
		if fs.Frame(name) == nil {
			return nil, fmt.Errorf("shared-frames constraint: frame %q not in frame system", name)
		}
	}
	posTol := sfc.PositionToleranceMM
	if posTol <= 0 {
		posTol = DefaultSharedFramesPositionToleranceMM
	}
	oriTolRad := utils.DegToRad(sfc.OrientationToleranceDegs)
	if oriTolRad <= 0 {
		oriTolRad = utils.DegToRad(DefaultSharedFramesOrientationToleranceDegs)
	}
	frames := append([]string{}, sfc.Frames...) // own the slice

	return func(state *StateFS) error {
		refPIF, err := state.FS.Transform(state.Configuration,
			referenceframe.NewZeroPoseInFrame(frames[0]), referenceframe.World)
		if err != nil {
			return err
		}
		refPose := refPIF.(*referenceframe.PoseInFrame).Pose()
		refPt := refPose.Point()
		refOri := refPose.Orientation()

		for _, name := range frames[1:] {
			pif, err := state.FS.Transform(state.Configuration,
				referenceframe.NewZeroPoseInFrame(name), referenceframe.World)
			if err != nil {
				return err
			}
			pose := pif.(*referenceframe.PoseInFrame).Pose()
			posErr := pose.Point().Sub(refPt).Norm()
			if posErr > posTol {
				return fmt.Errorf("%w: %s↔%s pos %.2fmm > tol %.2fmm",
					ErrSharedFramesConstraintViolated, frames[0], name, posErr, posTol)
			}
			diff := spatialmath.OrientationBetween(refOri, pose.Orientation())
			oriErrRad := math.Abs(diff.AxisAngles().Theta)
			if oriErrRad > oriTolRad {
				return fmt.Errorf("%w: %s↔%s ori %.2f° > tol %.2f°",
					ErrSharedFramesConstraintViolated, frames[0], name,
					utils.RadToDeg(oriErrRad), utils.RadToDeg(oriTolRad))
			}
		}
		return nil
	}, nil
}


// AddSharedFramesConstraint registers a shared-frames check with the checker's
// topological constraint chain. Follows the same pattern as AddCableConstraint:
// must be called after the checker is fully constructed.
func (c *ConstraintChecker) AddSharedFramesConstraint(
	fs *referenceframe.FrameSystem,
	sfc SharedFramesConstraint,
) error {
	check, err := newSharedFramesConstraintCheck(fs, sfc)
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
