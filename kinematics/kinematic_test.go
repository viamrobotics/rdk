package kinematics

import (
	"math"
	"testing"

	pb "go.viam.com/core/proto/api/v1"
	"go.viam.com/core/utils"

	"go.viam.com/test"
	"gonum.org/v1/gonum/num/quat"
)

func poseToSlice(p *pb.ArmPosition) []float64 {
	return []float64{p.X, p.Y, p.Z, p.Theta, p.OX, p.OY, p.OZ}
}

// This should test forward kinematics functions
func TestForwardKinematics(t *testing.T) {
	// Test fake 5DOF arm to confirm kinematics works with non-6dof arms
	m, err := ParseJSONFile(utils.ResolveFile("robots/wx250s/wx250s_test.json"))
	test.That(t, err, test.ShouldBeNil)

	// Confirm end effector starts at 300, 0, 360.25
	expect := []float64{300, 0, 360.25, 0, 1, 0, 0}
	actual := poseToSlice(ComputePosition(m, &pb.JointPositions{Degrees: []float64{0, 0, 0, 0, 0, 0}}))

	test.That(t, floatDelta(expect, actual), test.ShouldBeLessThanOrEqualTo, 0.00001)

	// Test the 6dof arm we actually have
	m, err = ParseJSONFile(utils.ResolveFile("robots/wx250s/wx250s_kinematics.json"))
	test.That(t, err, test.ShouldBeNil)

	// Confirm end effector starts at 365, 0, 360.25
	expect = []float64{365, 0, 360.25, 0, 1, 0, 0}
	actual = poseToSlice(ComputePosition(m, &pb.JointPositions{Degrees: []float64{0, 0, 0, 0, 0, 0}}))
	test.That(t, floatDelta(expect, actual), test.ShouldBeLessThanOrEqualTo, 0.00001)

	newPos := []float64{45, -45, 0, 0, 0, 0}
	actual = poseToSlice(ComputePosition(m, &pb.JointPositions{Degrees: newPos}))
	expect = []float64{57.5, 57.5, 545.1208197765168, 0, 0.5, 0.5, 0.707}
	test.That(t, floatDelta(expect, actual), test.ShouldBeLessThanOrEqualTo, 0.01)

	newPos = []float64{-45, 0, 0, 0, 0, 45}
	actual = poseToSlice(ComputePosition(m, &pb.JointPositions{Degrees: newPos}))
	expect = []float64{258.0935, -258.0935, 360.25, utils.RadToDeg(0.7854), 0.707, -0.707, 0}
	test.That(t, floatDelta(expect, actual), test.ShouldBeLessThanOrEqualTo, 0.01)
}

func floatDelta(l1, l2 []float64) float64 {
	delta := 0.0
	for i, v := range l1 {
		delta += math.Abs(v - l2[i])
	}
	return delta
}

const derivEqualityEpsilon = 1e-16

func derivComponentAlmostEqual(left, right float64) bool {
	return math.Abs(left-right) <= derivEqualityEpsilon
}

func areDerivsEqual(q1, q2 []quat.Number) bool {
	if len(q1) != len(q2) {
		return false
	}
	for i, dq1 := range q1 {
		dq2 := q2[i]
		if !derivComponentAlmostEqual(dq1.Real, dq2.Real) {
			return false
		}
		if !derivComponentAlmostEqual(dq1.Imag, dq2.Imag) {
			return false
		}
		if !derivComponentAlmostEqual(dq1.Jmag, dq2.Jmag) {
			return false
		}
		if !derivComponentAlmostEqual(dq1.Kmag, dq2.Kmag) {
			return false
		}
	}
	return true
}

func TestDeriv(t *testing.T) {
	// Test identity quaternion
	q := quat.Number{1, 0, 0, 0}
	qDeriv := []quat.Number{{0, 1, 0, 0}, {0, 0, 1, 0}, {0, 0, 0, 1}}

	match := areDerivsEqual(qDeriv, deriv(q))
	test.That(t, match, test.ShouldBeTrue)

	// Test non-identity single-axis unit quaternion
	q = quat.Exp(quat.Number{0, 2, 0, 0})

	qDeriv = []quat.Number{{-0.9092974268256816, -0.4161468365471424, 0, 0},
		{0, 0, 0.4546487134128408, 0},
		{0, 0, 0, 0.4546487134128408}}

	match = areDerivsEqual(qDeriv, deriv(q))
	test.That(t, match, test.ShouldBeTrue)

	// Test non-identity multi-axis unit quaternion
	q = quat.Exp(quat.Number{0, 2, 1.5, 0.2})

	qDeriv = []quat.Number{{-0.472134934000233, -0.42654977821280804, -0.4969629339096933, -0.06626172452129245},
		{-0.35410120050017474, -0.4969629339096933, -0.13665473343215354, -0.049696293390969336},
		{-0.0472134934000233, -0.06626172452129245, -0.049696293390969336, 0.22944129454798728}}

	match = areDerivsEqual(qDeriv, deriv(q))
	test.That(t, match, test.ShouldBeTrue)
}

func TestInline(t *testing.T) {
	// Test the 6dof arm we actually have
	m, err := ParseJSONFile(utils.ResolveFile("robots/wx250s/wx250s_kinematics.json"))
	test.That(t, err, test.ShouldBeNil)

	// The wx250s has the 4th and 6th joints inline
	zeroed := ZeroInlineRotation(m, []float64{0, 0, 0, -1, 0, 1})
	test.That(t, zeroed, test.ShouldResemble, []float64{0, 0, 0, 0, 0, 0})
}
