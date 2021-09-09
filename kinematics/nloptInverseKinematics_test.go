package kinematics

import (
	"context"
	"math"
	"testing"

	pb "go.viam.com/core/proto/api/v1"
	"go.viam.com/core/utils"

	"github.com/edaniels/golog"
	"go.viam.com/test"

	"go.viam.com/core/arm"
)

func TestCreateNloptIKSolver(t *testing.T) {
	logger := golog.NewTestLogger(t)
	m, err := ParseJSONFile(utils.ResolveFile("robots/wx250s/wx250s_kinematics.json"))
	test.That(t, err, test.ShouldBeNil)
	ik := CreateNloptIKSolver(m, logger, 1)

	pos := &pb.ArmPosition{X: 360, Z: 362}
	seed := arm.JointPositionsFromRadians([]float64{1, 1, 1, 1, 1, 0})

	_, err = ik.Solve(context.Background(), pos, seed)
	test.That(t, err, test.ShouldBeNil)

	pos = &pb.ArmPosition{X: -46, Y: -23, Z: 372, Theta: utils.RadToDeg(3.92), OX: -0.46, OY: 0.84, OZ: 0.28}

	seed = &pb.JointPositions{Degrees: []float64{49, 28, -101, 0, -73, 0}}

	_, err = ik.Solve(context.Background(), pos, seed)
	test.That(t, err, test.ShouldBeNil)
}

func TestNloptSwingReduction(t *testing.T) {
	startRadians := []float64{0, 0, 0, 0, 0, 0}
	endRadians := []float64{5, 0.5, 0.3, -0.2, 1.1, 2.3}
	expectRadians := []float64{5 - math.Pi, 0.5, 0.3, -0.2, 1.1, 2.3}

	swing, newRadians := checkExcessiveSwing(startRadians, endRadians, 2.8)
	test.That(t, swing, test.ShouldBeTrue)
	for i, val := range newRadians {
		test.That(t, val, test.ShouldAlmostEqual, expectRadians[i])
	}

	swing, newRadians = checkExcessiveSwing(startRadians, expectRadians, 2.8)
	test.That(t, swing, test.ShouldBeFalse)
	test.That(t, newRadians, test.ShouldResemble, expectRadians)
}

func TestInterpolateJoints(t *testing.T) {
	jp1 := &pb.JointPositions{Degrees: []float64{0, 4}}
	jp2 := &pb.JointPositions{Degrees: []float64{8, -8}}
	jpHalf := &pb.JointPositions{Degrees: []float64{4, -2}}
	jpQuarter := &pb.JointPositions{Degrees: []float64{2, 1}}

	interp1 := interpolateJoints(jp1, jp2, 0.5)
	interp2 := interpolateJoints(jp1, jp2, 0.25)
	test.That(t, interp1, test.ShouldResemble, jpHalf)
	test.That(t, interp2, test.ShouldResemble, jpQuarter)
}
