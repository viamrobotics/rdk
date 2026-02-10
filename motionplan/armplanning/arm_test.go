package armplanning_test

import (
	"context"
	"testing"

	"github.com/golang/geo/r3"
	"go.viam.com/test"

	"go.viam.com/rdk/components/arm"
	"go.viam.com/rdk/components/arm/fake"
	_ "go.viam.com/rdk/components/register"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/motionplan/armplanning"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/resource"
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
	OOBFloats := []referenceframe.Input{0, 0, 0, 0, 0, 720}
	injectedArm := &inject.Arm{
		Arm: notReal,
		JointPositionsFunc: func(ctx context.Context, extra map[string]interface{}) ([]referenceframe.Input, error) {
			return OOBFloats, nil
		},
		CurrentInputsFunc: func(ctx context.Context) ([]referenceframe.Input, error) {
			return OOBFloats, nil
		},
	}

	t.Run("MoveArm fails when OOB", func(t *testing.T) {
		pose := spatialmath.NewPoseFromPoint(r3.Vector{200, 200, 200})
		err := armplanning.MoveArm(context.Background(), logger, injectedArm, pose)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, referenceframe.OOBErrString)
	})

	t.Run("MoveToJointPositions fails OOB and moving further OOB", func(t *testing.T) {
		err := injectedArm.MoveToJointPositions(context.Background(), []referenceframe.Input{0, 0, 0, 0, 0, 900}, nil)
		test.That(t, err, test.ShouldNotBeNil)
	})

	t.Run("MoveToJointPositions succeeds when OOB and moving further in-bounds", func(t *testing.T) {
		err := injectedArm.MoveToJointPositions(context.Background(), []referenceframe.Input{0, 0, 0, 0, 0, 0}, nil)
		test.That(t, err, test.ShouldBeNil)
	})
}
