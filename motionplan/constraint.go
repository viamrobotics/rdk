package motionplan

import (
	"errors"
	"math"

	"gonum.org/v1/gonum/num/quat"

	"go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/utils"
)

// ErrOrientationConstraintViolated tags every orientation-tolerance failure
// (from both OrientationConstraint and LinearConstraint checks) so callers
// can tell an orientation violation apart from a collision with errors.Is
// instead of matching message text.
var ErrOrientationConstraintViolated = errors.New("orientation constraint violated")

// Constraints is a struct to store the constraints imposed upon a robot
// It serves as a convenenient RDK wrapper for the protobuf object.
type Constraints struct {
	LinearConstraint       []LinearConstraint       `json:"linear_constraints"`
	PseudolinearConstraint []PseudolinearConstraint `json:"pseudolinear_constraints"`
	OrientationConstraint  []OrientationConstraint  `json:"orientation_constraints"`
	CollisionSpecification []CollisionSpecification `json:"collision_specifications"`
}

// NewEmptyConstraints creates a new, empty Constraints object.
func NewEmptyConstraints() *Constraints {
	return &Constraints{
		LinearConstraint:       make([]LinearConstraint, 0),
		PseudolinearConstraint: make([]PseudolinearConstraint, 0),
		OrientationConstraint:  make([]OrientationConstraint, 0),
		CollisionSpecification: make([]CollisionSpecification, 0),
	}
}

// NewConstraints initializes a Constraints object with user-defined LinearConstraint, OrientationConstraint, and CollisionSpecification.
func NewConstraints(
	linConstraints []LinearConstraint,
	pseudoConstraints []PseudolinearConstraint,
	orientConstraints []OrientationConstraint,
	collSpecifications []CollisionSpecification,
) *Constraints {
	return &Constraints{
		LinearConstraint:       linConstraints,
		PseudolinearConstraint: pseudoConstraints,
		OrientationConstraint:  orientConstraints,
		CollisionSpecification: collSpecifications,
	}
}

// LinearConstraint specifies that the components being moved should move linearly relative to their goals.
type LinearConstraint struct {
	LineToleranceMm          float64 // Max linear deviation from straight-line between start and goal, in mm.
	OrientationToleranceDegs float64
}

// PseudolinearConstraint specifies that the component being moved should not deviate from the straight-line path to their goal by
// more than a factor proportional to the distance from start to goal.
// For example, if a component is moving 100mm, then a LineToleranceFactor of 1.0 means that the component will remain within a 100mm
// radius of the straight-line start-goal path.
type PseudolinearConstraint struct {
	LineToleranceFactor        float64
	OrientationToleranceFactor float64
}

// OrientationConstraint specifies that the components being moved will not deviate orientation beyond some threshold.
type OrientationConstraint struct {
	OrientationToleranceDegs float64
}

// Score computes a score which is how close we are to valid in degrees
func (oc *OrientationConstraint) Score(from, to, now spatialmath.Orientation) float64 {
	d := oc.Distance(from, to, now)
	if d <= 0 {
		return 0
	}
	return max(0, d-oc.OrientationToleranceDegs)
}

// Distance measures, in degrees, how far `now` strays from the direct
// reorientation between `from` and `to`: the angular distance from `now` to
// the nearest orientation on the geodesic (slerp) arc connecting the
// endpoints. The feasible band is therefore a connected tube of width
// OrientationToleranceDegs around the direct path. (This replaced a
// min-distance-to-either-endpoint rule whose feasible set split into two
// disconnected balls whenever the endpoints were more than twice the
// tolerance apart, making such reorientations unplannable.)
func (oc *OrientationConstraint) Distance(from, to, now spatialmath.Orientation) float64 {
	return newOrientationArc(from, to).distanceDegs(now)
}

// orientationArc is the geodesic arc between two orientations, precomputed as
// an orthonormal basis of its plane in quaternion space so point-to-arc
// distance is a couple of dot products per query.
type orientationArc struct {
	qf, u quat.Number // qf: arc start; u: unit vector orthogonal to qf in the arc plane
	omega float64     // arc angle in quaternion space (half the rotation angle), radians
	from  spatialmath.Orientation
}

func quatDot(a, b quat.Number) float64 {
	return a.Real*b.Real + a.Imag*b.Imag + a.Jmag*b.Jmag + a.Kmag*b.Kmag
}

func newOrientationArc(from, to spatialmath.Orientation) orientationArc {
	qf := from.Quaternion()
	qt := to.Quaternion()
	// q and -q are the same orientation; align signs to take the short arc.
	d := quatDot(qf, qt)
	if d < 0 {
		qt = quat.Scale(-1, qt)
		d = -d
	}
	arc := orientationArc{qf: qf, from: from}
	// Residual of qt orthogonal to qf; its norm is sin(omega).
	r := quat.Sub(qt, quat.Scale(d, qf))
	rn := math.Sqrt(quatDot(r, r))
	if rn < 1e-9 {
		return arc // from == to: the arc is a point
	}
	arc.u = quat.Scale(1/rn, r)
	arc.omega = math.Acos(min(d, 1))
	return arc
}

// distanceDegs returns the angular distance in degrees from now to the
// nearest orientation on the arc.
func (a orientationArc) distanceDegs(now spatialmath.Orientation) float64 {
	if a.omega == 0 {
		return OrientDist(a.from, now)
	}
	qn := now.Quaternion()
	x := quatDot(qn, a.qf)
	y := quatDot(qn, a.u)
	// The arc is q(t) = qf*cos(t) + u*sin(t), t in [0, omega], so
	// dot(qn, q(t)) = x*cos(t) + y*sin(t) = R*cos(t - phi). The angular
	// distance to q(t) is 2*acos(|dot|); maximize |dot| over the arc.
	best := max(math.Abs(x), math.Abs(x*math.Cos(a.omega)+y*math.Sin(a.omega)))
	phi := math.Atan2(y, x)
	for _, peak := range []float64{phi, phi + math.Pi, phi - math.Pi} {
		if peak > 0 && peak < a.omega {
			best = math.Hypot(x, y)
			break
		}
	}
	return utils.RadToDeg(2 * math.Acos(min(best, 1)))
}

// OrientationConstraintEval evaluates one OrientationConstraint against fixed
// start and goal orientations. The endpoints never change during a plan, so
// the geodesic-arc basis is precomputed once, leaving two dot products per
// check.
type OrientationConstraintEval struct {
	oc       OrientationConstraint
	from, to spatialmath.Orientation
	arc      orientationArc
}

// NewOrientationConstraintEval precomputes the fixed endpoint conversions.
func NewOrientationConstraintEval(oc OrientationConstraint, from, to spatialmath.Orientation) *OrientationConstraintEval {
	return &OrientationConstraintEval{
		oc: oc, from: from, to: to,
		arc: newOrientationArc(from, to),
	}
}

// Distance is OrientationConstraint.Distance with the arc precomputation amortized.
func (e *OrientationConstraintEval) Distance(now spatialmath.Orientation) float64 {
	return e.arc.distanceDegs(now)
}

// Score mirrors OrientationConstraint.Score.
func (e *OrientationConstraintEval) Score(now spatialmath.Orientation) float64 {
	d := e.Distance(now)
	if d <= 0 {
		return 0
	}
	return max(0, d-e.oc.OrientationToleranceDegs)
}

// CollisionSpecificationAllowedFrameCollisions is used to define frames that are allowed to collide.
type CollisionSpecificationAllowedFrameCollisions struct {
	Frame1, Frame2 string
}

// CollisionSpecification is used to selectively apply obstacle avoidance to specific parts of the robot.
type CollisionSpecification struct {
	// Pairs of frame which should be allowed to collide with one another
	Allows []CollisionSpecificationAllowedFrameCollisions
}

// AddLinearConstraint appends a LinearConstraint to a Constraints object.
func (c *Constraints) AddLinearConstraint(linConstraint LinearConstraint) {
	c.LinearConstraint = append(c.LinearConstraint, linConstraint)
}

// AddPseudolinearConstraint appends a PseudolinearConstraint to a Constraints object.
func (c *Constraints) AddPseudolinearConstraint(plinConstraint PseudolinearConstraint) {
	c.PseudolinearConstraint = append(c.PseudolinearConstraint, plinConstraint)
}

// AddOrientationConstraint appends a OrientationConstraint to a Constraints object.
func (c *Constraints) AddOrientationConstraint(orientConstraint OrientationConstraint) {
	c.OrientationConstraint = append(c.OrientationConstraint, orientConstraint)
}

// AddCollisionSpecification appends a CollisionSpecification to a Constraints object.
func (c *Constraints) AddCollisionSpecification(collConstraint CollisionSpecification) {
	c.CollisionSpecification = append(c.CollisionSpecification, collConstraint)
}
