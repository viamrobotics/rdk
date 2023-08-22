package ik

import (
	"math"
	"strings"

	"github.com/pkg/errors"
	pb "go.viam.com/api/component/arm/v1"
	"gonum.org/v1/gonum/floats"
	"gonum.org/v1/gonum/num/quat"

	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/spatialmath"
)

// ComputePosition takes a model and a protobuf JointPositions in degrees and returns the cartesian position of the
// end effector as a protobuf ArmPosition. This is performed statelessly without changing any data.
func ComputePosition(model referenceframe.Frame, joints *pb.JointPositions) (spatialmath.Pose, error) {
	if len(joints.Values) != len(model.DoF()) {
		return nil, errors.Errorf(
			"incorrect number of joints passed to ComputePosition. Want: %d, got: %d",
			len(model.DoF()),
			len(joints.Values),
		)
	}

	pose, err := model.Transform(model.InputFromProtobuf(joints))
	if err != nil {
		return nil, err
	}

	return pose, nil
}

// ComputeOOBPosition takes a model and a protobuf JointPositions in degrees and returns the cartesian
// position of the end effector as a protobuf ArmPosition even when the arm is in an out of bounds state.
// This is performed statelessly without changing any data.
func ComputeOOBPosition(model referenceframe.Frame, joints *pb.JointPositions) (spatialmath.Pose, error) {
	if joints == nil {
		return nil, referenceframe.ErrNilJointPositions
	}
	if model == nil {
		return nil, referenceframe.ErrNilModelFrame
	}

	if len(joints.Values) != len(model.DoF()) {
		return nil, errors.Errorf(
			"incorrect number of joints passed to ComputePosition. Want: %d, got: %d",
			len(model.DoF()),
			len(joints.Values),
		)
	}

	pose, err := model.Transform(model.InputFromProtobuf(joints))
	if err != nil && !strings.Contains(err.Error(), referenceframe.OOBErrString) {
		return nil, err
	}

	return pose, nil
}

// deriv will compute D(q), the derivative of q = e^w with respect to w
// Note that for prismatic joints, this will need to be expanded to dual quaternions.
func deriv(q quat.Number) []quat.Number {
	w := quat.Log(q)

	qNorm := math.Sqrt(w.Imag*w.Imag + w.Jmag*w.Jmag + w.Kmag*w.Kmag)
	// qNorm hits a singularity every 2pi
	// But if we flip the axis we get the same rotation but away from a singularity

	var quatD []quat.Number

	// qNorm is non-zero if our joint has a non-zero rotation
	if qNorm > 0 {
		b := math.Sin(qNorm) / qNorm
		c := (math.Cos(qNorm) / (qNorm * qNorm)) - (math.Sin(qNorm) / (qNorm * qNorm * qNorm))

		quatD = append(quatD, quat.Number{
			Real: -1 * w.Imag * b,
			Imag: b + w.Imag*w.Imag*c,
			Jmag: w.Imag * w.Jmag * c,
			Kmag: w.Imag * w.Kmag * c,
		})
		quatD = append(quatD, quat.Number{
			Real: -1 * w.Jmag * b,
			Imag: w.Jmag * w.Imag * c,
			Jmag: b + w.Jmag*w.Jmag*c,
			Kmag: w.Jmag * w.Kmag * c,
		})
		quatD = append(quatD, quat.Number{
			Real: -1 * w.Kmag * b,
			Imag: w.Kmag * w.Imag * c,
			Jmag: w.Kmag * w.Jmag * c,
			Kmag: b + w.Kmag*w.Kmag*c,
		})
	} else {
		quatD = append(quatD, quat.Number{0, 1, 0, 0})
		quatD = append(quatD, quat.Number{0, 0, 1, 0})
		quatD = append(quatD, quat.Number{0, 0, 0, 1})
	}
	return quatD
}

// L2Distance returns the L2 normalized difference between two equal length arrays.
func L2Distance(q1, q2 []float64) float64 {
	for i := 0; i < len(q1); i++ {
		q1[i] -= q2[i]
	}
	// 2 is the L value returning a standard L2 Normalization
	return floats.Norm(q1, 2)
}
