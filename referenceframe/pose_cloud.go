package referenceframe

import (
	"math"

	commonpb "go.viam.com/api/common/v1"

	"go.viam.com/rdk/spatialmath"
)

// PoseCloud can express leeway in individual dimensions with respect to a goal pose. Combined,
// these leeways describe a cloud where arriving at any destination within that cloud are considered
// equivalent.
//
// All of the leeways affect the algorithm independently. None of the leeways will be scaled based
// on how close a candidate pose is to the goal pose. Consider a case where a gripper wants to pick
// up a cup. There may be some freedom with respect to the exact orientation and theta of the
// gripper. But be cautious that there can be a scenario where a legal, but severe orientation
// leeway might work with a smaller difference in the candidate's theta (the cup stays mostly facing
// up), but not necessarily a larger one (the cup starts to tip and spill).
type PoseCloud struct {
	// A note about translational leeways (X, Y, Z) and orientations:
	//
	// Consider a case where an arm with a hand is pushing a block up a hill. It would be important
	// for the arm to follow the slope of the incline (the hand stays close to the incline). But it
	// might be okay if the hand, at any given waypoint, moves a little bit more up or down the
	// hill:
	//
	//     +------+
	//     |     /|
	//     |    / |
	//     | H◆/  |
	//     | /    |
	//     |/     |
	// X > +------+
	//     ^
	//     Z
	//
	// If we solve IK for a pose that's a bit "to the left" or down the hill, it's important that we
	// also lower the hand closer to the incline. If we try to calculate this in the world reference
	// frame, this means that the leeway for Z is in terms of a specific IK solution's leeway for X.
	//
	// To solve this, we always evaluate in the reference frame of the object being asked to move
	// towards. In the above example, a movement request would ask the hand to move to a
	// `PoseInFrame` where the block is the reference frame. Hence, the leeways would be applied to
	// the hand in the reference frame of the block (which is laying flat on the slope). For this to
	// work, we would want the block's orientation vector to be either perpendicular or parallel to
	// the incline. Such that (in the case of perpendicular) we declare the leeway of Z' to be
	// 0. While the leeway for X' (in the block reference frame) can be a wider range.

	// The following X, Y and Z are translational leeways. They are all in units of millimeters, the
	// same as a goal pose. The value represents a leeway in the range of [-Value, +Value].
	//
	// The X leeway where any X (with respect to the movement object's orientation).
	X float64 `json:"x"`
	// The Y leeway where any Y (with respect to the movement object's orientation).
	Y float64 `json:"y"`
	// The Z leeway where any Z (with respect to the movement object's orientation).
	Z float64 `json:"z"`

	// The following orientation leeway values represents a leeway in the range of [-Value,
	// +Value]. The orientation values are unitless, but one must keep in mind they are applied to
	// an orientation vector that has been normalized to a unit sphere. For example, an OX leeway of
	// `1` would accept any OX for a candidate pose.
	//
	// OX represents the leeway as described above.
	OX float64 `json:"ox"`
	// OY represents the leeway as described above.
	OY float64 `json:"oy"`
	// OZ represents the leeway as described above.
	OZ float64 `json:"oz"`

	// Theta represents the [-Theta, +Theta] in an objects rotation around its
	// orientation axis in the unit of degrees.
	Theta float64 `json:"theta"`
}

// PoseInCloud returns true if the `candidatePose` is within this cloud of the `goalPose`.
func (pc *PoseCloud) PoseInCloud(goalPose, candidatePose spatialmath.Pose) bool {
	// Default distance below which two distances are considered equal. Copied from `ik` package to
	// avoid package cycles. This is only necessary for the default leeway of `0` to not dismiss
	// every candidate.
	const defaultEpsilon = 0.001

	between := spatialmath.PoseBetween(goalPose, candidatePose)
	if math.Abs(between.Point().X) > pc.X+defaultEpsilon {
		return false
	}
	if math.Abs(between.Point().Y) > pc.Y+defaultEpsilon {
		return false
	}
	if math.Abs(between.Point().Z) > pc.Z+defaultEpsilon {
		return false
	}

	betweenOrientation := between.Orientation().OrientationVectorDegrees()
	if math.Abs(betweenOrientation.OX) > pc.OX+defaultEpsilon {
		return false
	}
	if math.Abs(betweenOrientation.OY) > pc.OY+defaultEpsilon {
		return false
	}
	if math.Abs(1-betweenOrientation.OZ) > pc.OZ+defaultEpsilon {
		return false
	}

	return true
}

// ToProto turns this to proto.
func (pc *PoseCloud) ToProto() *commonpb.PoseCloud {
	if pc == nil {
		// We return nil here to minimize tests that need changing. Nil is logically equivalent to a
		// default constructed `PoseCloud` (all leeways are zero), but test assertions make a
		// distinction.
		return nil
	}

	return &commonpb.PoseCloud{
		X:     pc.X,
		Y:     pc.Y,
		Z:     pc.Z,
		OX:    pc.OX,
		OY:    pc.OY,
		OZ:    pc.OZ,
		Theta: pc.Theta,
	}
}
