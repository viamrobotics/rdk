package motionplan

import (
	"errors"

	motionpb "go.viam.com/api/service/motion/v1"

	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/spatialmath"
)

// FrameSystemPosesToProto converts a referenceframe.FrameSystemPoses to its representation in protobuf.
func FrameSystemPosesToProto(ps referenceframe.FrameSystemPoses) *motionpb.PlanStep {
	step := make(map[string]*motionpb.ComponentState)
	for name, pose := range ps {
		pbPose := spatialmath.PoseToProtobuf(pose.Pose())
		step[name] = &motionpb.ComponentState{Pose: pbPose}
	}
	return &motionpb.PlanStep{Step: step}
}

// FrameSystemPosesFromProto converts a *pb.PlanStep to a PlanStep.
func FrameSystemPosesFromProto(ps *motionpb.PlanStep) (referenceframe.FrameSystemPoses, error) {
	if ps == nil {
		return referenceframe.FrameSystemPoses{}, errors.New("received nil *pb.PlanStep")
	}

	step := make(referenceframe.FrameSystemPoses, len(ps.Step))
	for k, v := range ps.Step {
		step[k] = referenceframe.NewPoseInFrame(referenceframe.World, spatialmath.NewPoseFromProtobuf(v.Pose))
	}
	return step, nil
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
