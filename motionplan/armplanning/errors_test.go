package armplanning

import (
	"testing"

	"github.com/pkg/errors"
	"go.viam.com/test"
)

func TestNewIKConstraintErr(t *testing.T) {
	constraintFailCnt := 10000

	t.Run("some errors exceed cutoff", func(t *testing.T) {
		failures := map[error]int{
			errors.New("obstacle constraint: violation between ur5e-modular:forearm_link and box1 geometries"):                  2913,
			errors.New("self-collision constraint: violation between gripper-1:clamp and ur5e-modular:wrist_1_link geometries"): 97,
		}

		ikError := newIkConstraintError(nil, nil)
		for err, cnt := range failures {
			for range cnt {
				ikError.add(nil, err)
			}
		}
		ikError.Count = constraintFailCnt

		expectedError := errors.New("all IK solutions failed constraints. Failures: " +
			"{ obstacle constraint: violation between ur5e-modular:forearm_link and box1 geometries: 29.13% }, ")
		test.That(t, ikError, test.ShouldBeError, expectedError)
	})

	t.Run("no errors exceed cutoff", func(t *testing.T) {
		failures := map[error]int{
			errors.New("self-collision constraint: violation between gripper-1:clamp and ur5e-modular:forearm_link geometries"): 291,
			errors.New("self-collision constraint: violation between gripper-1:clamp and ur5e-modular:wrist_1_link geometries"): 97,
		}

		ikError := newIkConstraintError(nil, nil)
		for err, cnt := range failures {
			for range cnt {
				ikError.add(nil, err)
			}
		}
		ikError.Count = constraintFailCnt

		expectedError := errors.New("all IK solutions failed constraints. Failures: " +
			"{ self-collision constraint: violation between gripper-1:clamp and ur5e-modular:forearm_link geometries: 2.91% }, " +
			"{ self-collision constraint: violation between gripper-1:clamp and ur5e-modular:wrist_1_link geometries: 0.97% }, ")
		test.That(t, ikError, test.ShouldBeError, expectedError)
	})
}
