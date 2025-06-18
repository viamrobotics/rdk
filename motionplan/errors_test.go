package motionplan

import (
	"testing"

	"github.com/pkg/errors"
	"go.viam.com/test"
)

func TestNewIKConstraintErr(t *testing.T) {
	constraintFailCnt := 10000

	t.Run("some errors exceed cutoff", func(t *testing.T) {
		failures := map[string]int{
			"obstacle constraint: violation between ur5e-modular:forearm_link and box1 geometries":                  2913,
			"self-collision constraint: violation between gripper-1:clamp and ur5e-modular:wrist_1_link geometries": 97,
		}

		//nolint:lll
		expectedError := errors.New("all IK solutions failed constraints. Failures: { obstacle constraint: violation between ur5e-modular:forearm_link and box1 geometries: 29.13% }, ")
		test.That(t, newIKConstraintErr(failures, constraintFailCnt), test.ShouldBeError, expectedError)
	})

	t.Run("no errors exceed cutoff", func(t *testing.T) {
		failures := map[string]int{
			"self-collision constraint: violation between gripper-1:clamp and ur5e-modular:forearm_link geometries": 291,
			"self-collision constraint: violation between gripper-1:clamp and ur5e-modular:wrist_1_link geometries": 97,
		}

		//nolint:lll
		expectedError := errors.New("all IK solutions failed constraints. Failures: { self-collision constraint: violation between gripper-1:clamp and ur5e-modular:forearm_link geometries: 2.91% }, { self-collision constraint: violation between gripper-1:clamp and ur5e-modular:wrist_1_link geometries: 0.97% }, ")
		test.That(t, newIKConstraintErr(failures, constraintFailCnt), test.ShouldBeError, expectedError)
	})
}
