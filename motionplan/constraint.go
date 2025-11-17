package motionplan

import (
	"go.viam.com/rdk/spatialmath"
)

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

func between(a, b, v float64) bool {
	if a > b {
		a, b = b, a
	}
	return v >= a && v <= b
}

// Score computes a score which is how close we are to valid in degrees
func (oc *OrientationConstraint) Score(from, to, now spatialmath.Orientation) float64 {
	d := oc.Distance(from, to, now)
	if d <= 0 {
		return 0
	}
	return max(0, d-oc.OrientationToleranceDegs)
}

// Distance apart in degrees
func (oc *OrientationConstraint) Distance(from, to, now spatialmath.Orientation) float64 {
	f := from.OrientationVectorDegrees()
	t := to.OrientationVectorDegrees()
	n := now.OrientationVectorDegrees()

	if between(f.OX, t.OX, n.OX) &&
		between(f.OY, t.OY, n.OY) &&
		between(f.OZ, t.OZ, n.OZ) &&
		between(f.Theta, t.Theta, n.Theta) {
		return 0
	}

	a := OrientDist(from, now)
	b := OrientDist(to, now)
	return min(a, b)
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
