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
// The difference is measured in the reference's own frame, as the orientation vector of the
// rotation taking the reference to the candidate. OX, OY and OZ therefore bound where the
// candidate's Z axis points relative to the reference's, and Theta bounds the spin about that axis.
// A cloud of {OX: 0.17, OY: 0.17, OZ: 0.015, Theta: 180} admits any spin about the reference Z axis
// while keeping that axis within ~10 degrees of the reference - a cup that must stay upright.
//
// Theta inherits the orientation vector's convention, which is gimbal-locked at the pole: once the
// Z axis leans by more than a fraction of a degree, theta reads as the azimuth of that lean (0
// toward +X, ∓90 toward ±Y, 180 toward -X) plus any spin, whatever the lean's size. A Theta leeway
// below 180 combined with a nonzero tilt leeway therefore admits a lean toward some directions and
// rejects the same lean toward others; bound the tilt alone (Theta: 180) or the spin alone (zero
// tilt leeways) unless that is intended.
type OrientationCloud struct {
	// OX, OY and OZ are unitless leeways on the components of the unit orientation vector, each
	// accepting the range [-Value, +Value] around the reference (whose own vector is (0, 0, 1),
	// so the OZ leeway is measured from 1). A leeway of 1 accepts any value of that component.
	OX float64 `json:"ox"`
	OY float64 `json:"oy"`
	OZ float64 `json:"oz"`
	// Theta is the leeway, in degrees, on the rotation about the orientation vector, accepting
	// the range [-Theta, +Theta].
	Theta float64 `json:"theta"`
}

// OrientationInCloud reports whether candidate lies within this cloud of reference.
func (oc *OrientationCloud) OrientationInCloud(reference, candidate spatialmath.Orientation) bool {
	return oc.inCloud(orientationBetweenLocal(reference, candidate))
}

// Excess measures, in degrees, how far candidate lies outside this cloud of reference: zero
// inside, otherwise the sum of each violated leeway's angular overshoot. Unlike
// OrientationInCloud it applies no epsilon and is continuous across the boundary, so it can serve
// as a gradient-descent objective for pulling an orientation back into the cloud.
func (oc *OrientationCloud) Excess(reference, candidate spatialmath.Orientation) float64 {
	return oc.excessDegs(orientationBetweenLocal(reference, candidate))
}

// orientationBetweenLocal is the rotation taking reference to candidate expressed in the
// reference's own frame - the orientation half of spatialmath.PoseBetween, so that a cloud reads
// the same whether checked through a PoseCloud or on its own. (spatialmath.OrientationBetween
// gives the world-frame difference, whose orientation vector points somewhere else entirely.)
func orientationBetweenLocal(reference, candidate spatialmath.Orientation) *spatialmath.OrientationVectorDegrees {
	return spatialmath.QuatToOVD(quat.Mul(quat.Conj(reference.Quaternion()), candidate.Quaternion()))
}

// InscribedAngleDegs returns a rotation angle, in degrees, such that every orientation within that
// angle of the reference lies in the cloud: the radius of a ball of orientations inscribed in it.
func (oc *OrientationCloud) InscribedAngleDegs() float64 {
	// Theta is gimbal-locked at the pole (see OrientationCloud): past a fraction of a degree of
	// lean, theta reads as the lean's azimuth, so a lean toward -X of any size reads as 180 and
	// no ball fits inside a cloud that bounds theta at all. Otherwise a rotation by angle a tilts
	// the Z axis by at most a, and the tilt leeways convert directly to the angles they subtend.
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

// inCloud is OrientationInCloud on an already-computed reference-to-candidate orientation vector.
func (oc *OrientationCloud) inCloud(between *spatialmath.OrientationVectorDegrees) bool {
	return math.Abs(between.OX) <= oc.OX+cloudEpsilon &&
		math.Abs(between.OY) <= oc.OY+cloudEpsilon &&
		math.Abs(1-between.OZ) <= oc.OZ+cloudEpsilon &&
		math.Abs(between.Theta) <= oc.Theta+cloudEpsilon
}

// excessDegs is Excess on an already-computed reference-to-candidate orientation vector.
func (oc *OrientationCloud) excessDegs(between *spatialmath.OrientationVectorDegrees) float64 {
	// Each unit-vector leeway is converted to the angle it subtends so all four terms share a
	// degree scale: |OX| <= L bounds the sine of the axis' lean along X, and 1-OZ <= L bounds the
	// cosine of its total tilt.
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
