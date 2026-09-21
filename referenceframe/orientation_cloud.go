package referenceframe

import (
	"math"

	"gonum.org/v1/gonum/num/quat"

	"go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/utils"
)

// cloudEpsilon is the slack below which a candidate is considered to sit on a cloud boundary, so
// that a leeway of 0 still admits candidates that match the reference up to floating-point noise.
// Copied from the `ik` package's default epsilon to avoid a package cycle.
const cloudEpsilon = 0.001

// OrientationCloud expresses per-axis leeway around a reference orientation: the orientation half
// of a PoseCloud. Any candidate orientation whose difference from the reference falls within every
// leeway is considered equivalent to the reference.
//
// The difference is the orientation vector of the rotation taking the reference to the candidate,
// expressed in the reference's frame: OX, OY and OZ bound where the candidate's Z axis points and
// Theta bounds the spin about it. {OX: 0.17, OY: 0.17, OZ: 0.015, Theta: 180} keeps that axis
// within ~10 degrees of the reference while allowing any spin - a cup that must stay upright.
//
// Theta is gimbal-locked at the pole: past a fraction of a degree of lean it reads as the lean's
// azimuth (0 toward +X, ∓90 toward ±Y, 180 toward -X) plus any spin, regardless of the lean's
// size. A Theta below 180 alongside nonzero tilt leeways therefore admits a lean in some directions
// and rejects it in others; bound the tilt alone (Theta: 180) or the spin alone (zero tilt leeways).
type OrientationCloud struct {
	// OX, OY and OZ are unitless leeways of [-Value, +Value] on the orientation vector's components
	// around the reference's (0, 0, 1), so OZ is measured from 1; a leeway of 1 frees a component.
	OX float64 `json:"ox"`
	OY float64 `json:"oy"`
	OZ float64 `json:"oz"`
	// Theta is the leeway of [-Theta, +Theta] degrees on the rotation about the orientation vector.
	Theta float64 `json:"theta"`
}

// OrientationInCloud reports whether candidate lies within this cloud of reference.
func (oc *OrientationCloud) OrientationInCloud(reference, candidate spatialmath.Orientation) bool {
	return oc.inCloud(orientationBetweenLocal(reference, candidate))
}

// Excess measures, in degrees, how far candidate lies outside this cloud of reference: zero
// inside, otherwise the sum of each violated leeway's angular overshoot. It applies no epsilon and
// is continuous across the boundary, which makes it usable as a gradient-descent objective.
func (oc *OrientationCloud) Excess(reference, candidate spatialmath.Orientation) float64 {
	return oc.excessDegs(orientationBetweenLocal(reference, candidate))
}

// orientationBetweenLocal is the rotation taking reference to candidate in the reference's own
// frame, matching spatialmath.PoseBetween so a cloud reads the same through a PoseCloud or alone.
// spatialmath.OrientationBetween is the world-frame difference and would not.
func orientationBetweenLocal(reference, candidate spatialmath.Orientation) *spatialmath.OrientationVectorDegrees {
	return spatialmath.QuatToOVD(quat.Mul(quat.Conj(reference.Quaternion()), candidate.Quaternion()))
}

// InscribedAngleDegs returns a rotation angle, in degrees, such that every orientation within that
// angle of the reference lies in the cloud: the radius of a ball of orientations inscribed in it.
func (oc *OrientationCloud) InscribedAngleDegs() float64 {
	// A lean toward -X of any size reads as theta 180 (see OrientationCloud), so no ball fits in a
	// cloud that bounds theta at all. Otherwise a rotation by a tilts the Z axis by at most a, and
	// the tilt leeways bound the radius directly.
	if oc.Theta < 180 {
		return 0
	}
	tilt := math.Inf(1)
	if oc.OX < 1 {
		tilt = min(tilt, math.Asin(max(oc.OX, 0)))
	}
	if oc.OY < 1 {
		tilt = min(tilt, math.Asin(max(oc.OY, 0)))
	}
	if oc.OZ < 2 {
		tilt = min(tilt, math.Acos(1-max(oc.OZ, 0)))
	}
	return utils.RadToDeg(tilt)
}

func (oc *OrientationCloud) inCloud(between *spatialmath.OrientationVectorDegrees) bool {
	return math.Abs(between.OX) <= oc.OX+cloudEpsilon &&
		math.Abs(between.OY) <= oc.OY+cloudEpsilon &&
		math.Abs(1-between.OZ) <= oc.OZ+cloudEpsilon &&
		math.Abs(between.Theta) <= oc.Theta+cloudEpsilon
}

func (oc *OrientationCloud) excessDegs(between *spatialmath.OrientationVectorDegrees) float64 {
	// The unit-vector leeways bound a sine (OX, OY) or a cosine (OZ) of the lean; converting them
	// to the angles they subtend puts all four terms on one degree scale.
	excess := 0.0
	if ox := math.Abs(between.OX); ox > oc.OX {
		excess += math.Asin(min(ox, 1)) - math.Asin(min(max(oc.OX, 0), 1))
	}
	if oy := math.Abs(between.OY); oy > oc.OY {
		excess += math.Asin(min(oy, 1)) - math.Asin(min(max(oc.OY, 0), 1))
	}
	if 1-between.OZ > oc.OZ {
		excess += math.Acos(min(max(between.OZ, -1), 1)) - math.Acos(min(max(1-oc.OZ, -1), 1))
	}
	excess = utils.RadToDeg(excess)
	if theta := math.Abs(between.Theta); theta > oc.Theta {
		excess += theta - oc.Theta
	}
	return excess
}
