// Package kinematics implements various kinematics methods for use with
// robotic parts needing to describe motion.
package kinematics

import (
	"math"

	pb "go.viam.com/core/proto/api/v1"
	"go.viam.com/core/spatialmath"
	"go.viam.com/core/utils"

	"github.com/go-errors/errors"
	"github.com/go-gl/mathgl/mgl64"
	"gonum.org/v1/gonum/floats"
	"gonum.org/v1/gonum/num/quat"
)

// ComputePosition takes a model and a protobuf JointPositions in degrees and returns the cartesian position of the
// end effector as a protobuf ArmPosition. This is performed statelessly without changing any data.
func ComputePosition(model *Model, joints *pb.JointPositions) (*pb.ArmPosition, error) {

	if len(joints.Degrees) != model.Dof() {
		return nil, errors.Errorf("incorrect number of joints passed to ComputePosition. Want: %d, got: %d", model.Dof(), len(joints.Degrees))
	}

	radAngles := make([]float64, len(joints.Degrees))
	for i, angle := range joints.Degrees {
		radAngles[i] = utils.DegToRad(angle)
	}

	return JointRadToQuat(model, radAngles).ToArmPos(), nil
}

// JointRadToQuat takes a model and a list of joint angles in radians and computes the dual quaternion representing the
// cartesian position of the end effector. This is useful for when conversions between quaternions and OV are not needed.
func JointRadToQuat(model *Model, radAngles []float64) *spatialmath.DualQuaternion {
	quats := model.GetQuaternions(radAngles)
	// Start at ((1+0i+0j+0k)+(+0+0i+0j+0k)ϵ)
	startPos := spatialmath.NewDualQuaternion()
	for _, quat := range quats {
		startPos.Quat = startPos.Transformation(quat.Quat)
	}
	return startPos
}

// ZeroInlineRotation will look for joint angles that are approximately complementary (e.g. 0.5 and -0.5) and check if they
// are inline by seeing if moving both closer to zero changes the 6d position. If they appear to be inline it will set
// both to zero if they are not. This should avoid needless twists of inline joints.
func ZeroInlineRotation(m *Model, angles []float64) []float64 {
	epsilon := 0.0001

	newAngles := make([]float64, len(angles))
	copy(newAngles, angles)

	for i, angle1 := range angles {
		for j := i + 1; j < len(angles); j++ {
			angle2 := angles[j]
			if mgl64.FloatEqualThreshold(angle1*-1, angle2, epsilon) {
				tempAngles := make([]float64, len(angles))
				copy(tempAngles, angles)
				tempAngles[i] = 0
				tempAngles[j] = 0

				// These angles are complementary
				pos1 := JointRadToQuat(m, angles)
				pos2 := JointRadToQuat(m, tempAngles)
				distance := SquaredNorm(pos1.ToDelta(pos2))

				// Check we did not move the end effector too much
				if distance < epsilon*epsilon {
					newAngles[i] = 0
					newAngles[j] = 0
				}
			}
		}
	}
	return newAngles
}

// deriv will compute D(q), the derivative of q = e^w with respect to w
// Note that for prismatic joints, this will need to be expanded to dual quaternions
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

		quatD = append(quatD, quat.Number{Real: -1 * w.Imag * b,
			Imag: b + w.Imag*w.Imag*c,
			Jmag: w.Imag * w.Jmag * c,
			Kmag: w.Imag * w.Kmag * c})
		quatD = append(quatD, quat.Number{Real: -1 * w.Jmag * b,
			Imag: w.Jmag * w.Imag * c,
			Jmag: b + w.Jmag*w.Jmag*c,
			Kmag: w.Jmag * w.Kmag * c})
		quatD = append(quatD, quat.Number{Real: -1 * w.Kmag * b,
			Imag: w.Kmag * w.Imag * c,
			Jmag: w.Kmag * w.Jmag * c,
			Kmag: b + w.Kmag*w.Kmag*c})

	} else {
		quatD = append(quatD, quat.Number{0, 1, 0, 0})
		quatD = append(quatD, quat.Number{0, 0, 1, 0})
		quatD = append(quatD, quat.Number{0, 0, 0, 1})
	}
	return quatD
}

// L2Distance returns the L2 normalized difference between two equal length arrays
func L2Distance(q1, q2 []float64) float64 {
	for i := 0; i < len(q1); i++ {
		q1[i] = q1[i] - q2[i]
	}
	// 2 is the L value returning a standard L2 Normalization
	return floats.Norm(q1, 2)
}
