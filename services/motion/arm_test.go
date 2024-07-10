package motion_test

import (
	"context"
	"testing"

	"github.com/golang/geo/r3"
	pb "go.viam.com/api/component/arm/v1"
	"go.viam.com/test"

	"go.viam.com/rdk/components/arm"
	"go.viam.com/rdk/components/arm/fake"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/motionplan"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/services/motion"
	"go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/testutils/inject"
)

func TestOOBArmMotion(t *testing.T) {
	logger := logging.NewTestLogger(t)
	cfg := resource.Config{
		Name:  arm.API.String(),
		Model: resource.DefaultModelFamily.WithModel("ur5e"),
		ConvertedAttributes: &fake.Config{
			ArmModel: "ur5e",
		},
	}

	// instantiate out of bounds arm
	notReal, err := fake.NewArm(context.Background(), nil, cfg, logger)
	test.That(t, err, test.ShouldBeNil)
	injectedArm := &inject.Arm{
		Arm: notReal,
		JointPositionsFunc: func(ctx context.Context, extra map[string]interface{}) (*pb.JointPositions, error) {
			return &pb.JointPositions{Values: []float64{0, 0, 0, 0, 0, 720}}, nil
		},
	}

	t.Run("EndPosition works when OOB", func(t *testing.T) {
		jPositions := pb.JointPositions{Values: []float64{0, 0, 0, 0, 0, 720}}
		pose, err := motionplan.ComputeOOBPosition(injectedArm.ModelFrame(), &jPositions)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, pose, test.ShouldNotBeNil)
	})

	t.Run("MoveArm fails when OOB", func(t *testing.T) {
		pose := spatialmath.NewPoseFromPoint(r3.Vector{200, 200, 200})
		err := motion.MoveArm(context.Background(), logger, injectedArm, pose)
		test.That(t, err.Error(), test.ShouldContain, referenceframe.OOBErrString)
	})

	t.Run("MoveToJointPositions fails when OOB", func(t *testing.T) {
		err := injectedArm.MoveToJointPositions(context.Background(), &pb.JointPositions{Values: []float64{0, 0, 0, 0, 0, 0}}, nil)
		test.That(t, err.Error(), test.ShouldContain, referenceframe.OOBErrString)
	})
}
