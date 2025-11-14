package motionplan

import (
	motionpb "go.viam.com/api/service/motion/v1"

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

// ConstraintsFromProtobuf converts a protobuf object to a Constraints object.
func ConstraintsFromProtobuf(pbConstraint *motionpb.Constraints) *Constraints {
	if pbConstraint == nil {
		return NewEmptyConstraints()
	}

	// iterate through all motionpb.LinearConstraint and convert to RDK form
	linConstraintFromProto := func(linConstraints []*motionpb.LinearConstraint) []LinearConstraint {
		toRet := make([]LinearConstraint, 0, len(linConstraints))
		for _, linConstraint := range linConstraints {
			linTol := 0.
			if linConstraint.LineToleranceMm != nil {
				linTol = float64(*linConstraint.LineToleranceMm)
			}
			orientTol := 0.
			if linConstraint.OrientationToleranceDegs != nil {
				orientTol = float64(*linConstraint.OrientationToleranceDegs)
			}
			toRet = append(toRet, LinearConstraint{
				LineToleranceMm:          linTol,
				OrientationToleranceDegs: orientTol,
			})
		}
		return toRet
	}

	plinConstraintFromProto := func(plinConstraints []*motionpb.PseudolinearConstraint) []PseudolinearConstraint {
		toRet := make([]PseudolinearConstraint, 0, len(plinConstraints))
		for _, plc := range plinConstraints {
			linTol := 0.
			if plc.LineToleranceFactor != nil {
				linTol = float64(*plc.LineToleranceFactor)
			}
			orientTol := 0.
			if plc.OrientationToleranceFactor != nil {
				orientTol = float64(*plc.OrientationToleranceFactor)
			}
			toRet = append(toRet, PseudolinearConstraint{
				LineToleranceFactor:        linTol,
				OrientationToleranceFactor: orientTol,
			})
		}
		return toRet
	}

	// iterate through all motionpb.OrientationConstraint and convert to RDK form
	orientConstraintFromProto := func(orientConstraints []*motionpb.OrientationConstraint) []OrientationConstraint {
		toRet := make([]OrientationConstraint, 0, len(orientConstraints))
		for _, orientConstraint := range orientConstraints {
			orientTol := 0.
			if orientConstraint.OrientationToleranceDegs != nil {
				orientTol = float64(*orientConstraint.OrientationToleranceDegs)
			}
			toRet = append(toRet, OrientationConstraint{
				OrientationToleranceDegs: orientTol,
			})
		}
		return toRet
	}

	// iterate through all motionpb.CollisionSpecification and convert to RDK form
	collSpecFromProto := func(collSpecs []*motionpb.CollisionSpecification) []CollisionSpecification {
		toRet := make([]CollisionSpecification, 0, len(collSpecs))
		for _, collSpec := range collSpecs {
			allowedFrameCollisions := make([]CollisionSpecificationAllowedFrameCollisions, 0)
			for _, collSpecAllowedFrame := range collSpec.Allows {
				allowedFrameCollisions = append(allowedFrameCollisions, CollisionSpecificationAllowedFrameCollisions{
					Frame1: collSpecAllowedFrame.Frame1,
					Frame2: collSpecAllowedFrame.Frame2,
				})
			}
			toRet = append(toRet, CollisionSpecification{
				Allows: allowedFrameCollisions,
			})
		}
		return toRet
	}

	return NewConstraints(
		linConstraintFromProto(pbConstraint.LinearConstraint),
		plinConstraintFromProto(pbConstraint.PseudolinearConstraint),
		orientConstraintFromProto(pbConstraint.OrientationConstraint),
		collSpecFromProto(pbConstraint.CollisionSpecification),
	)
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

// ToProtobuf takes an existing Constraints object and converts it to a protobuf.
func (c *Constraints) ToProtobuf() *motionpb.Constraints {
	if c == nil {
		return nil
	}
	// convert LinearConstraint to motionpb.LinearConstraint
	convertLinConstraintToProto := func(linConstraints []LinearConstraint) []*motionpb.LinearConstraint {
		toRet := make([]*motionpb.LinearConstraint, 0)
		for _, linConstraint := range linConstraints {
			lineTolerance := float32(linConstraint.LineToleranceMm)
			orientationTolerance := float32(linConstraint.OrientationToleranceDegs)
			toRet = append(toRet, &motionpb.LinearConstraint{
				LineToleranceMm:          &lineTolerance,
				OrientationToleranceDegs: &orientationTolerance,
			})
		}
		return toRet
	}

	convertPseudoLinConstraintToProto := func(plinConstraints []PseudolinearConstraint) []*motionpb.PseudolinearConstraint {
		toRet := make([]*motionpb.PseudolinearConstraint, 0)
		for _, plc := range plinConstraints {
			lineTolerance := float32(plc.LineToleranceFactor)
			orientationTolerance := float32(plc.OrientationToleranceFactor)
			toRet = append(toRet, &motionpb.PseudolinearConstraint{
				LineToleranceFactor:        &lineTolerance,
				OrientationToleranceFactor: &orientationTolerance,
			})
		}
		return toRet
	}

	// convert OrientationConstraint to motionpb.OrientationConstraint
	convertOrientConstraintToProto := func(orientConstraints []OrientationConstraint) []*motionpb.OrientationConstraint {
		toRet := make([]*motionpb.OrientationConstraint, 0)
		for _, orientConstraint := range orientConstraints {
			orientationTolerance := float32(orientConstraint.OrientationToleranceDegs)
			toRet = append(toRet, &motionpb.OrientationConstraint{
				OrientationToleranceDegs: &orientationTolerance,
			})
		}
		return toRet
	}

	// convert CollisionSpecifications to motionpb.CollisionSpecification
	convertCollSpecToProto := func(collSpecs []CollisionSpecification) []*motionpb.CollisionSpecification {
		toRet := make([]*motionpb.CollisionSpecification, 0)
		for _, collSpec := range collSpecs {
			allowedFrameCollisions := make([]*motionpb.CollisionSpecification_AllowedFrameCollisions, 0)
			for _, collSpecAllowedFrame := range collSpec.Allows {
				allowedFrameCollisions = append(allowedFrameCollisions, &motionpb.CollisionSpecification_AllowedFrameCollisions{
					Frame1: collSpecAllowedFrame.Frame1,
					Frame2: collSpecAllowedFrame.Frame2,
				})
			}
			toRet = append(toRet, &motionpb.CollisionSpecification{
				Allows: allowedFrameCollisions,
			})
		}
		return toRet
	}

	return &motionpb.Constraints{
		LinearConstraint:       convertLinConstraintToProto(c.LinearConstraint),
		PseudolinearConstraint: convertPseudoLinConstraintToProto(c.PseudolinearConstraint),
		OrientationConstraint:  convertOrientConstraintToProto(c.OrientationConstraint),
		CollisionSpecification: convertCollSpecToProto(c.CollisionSpecification),
	}
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
