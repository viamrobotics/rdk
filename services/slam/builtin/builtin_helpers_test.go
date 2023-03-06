package builtin

import (
	"math"
	"testing"

	"github.com/golang/geo/r3"
	spatial "go.viam.com/rdk/spatialmath"
	"go.viam.com/test"
)

func TestBuiltinQuaternion(t *testing.T) {
	t.Run("test successful quaternion from internal server", func(t *testing.T) {

		poseSucc := spatial.NewPose(r3.Vector{X: 1, Y: 2, Z: 3}, &spatial.OrientationVector{Theta: math.Pi / 2, OX: 0, OY: 0, OZ: -1})
		componentRefSucc := "cam"
		returnedExtSucc := map[string]interface{}{
			"quat": map[string]interface{}{
				"real": poseSucc.Orientation().Quaternion().Real,
				"imag": poseSucc.Orientation().Quaternion().Imag,
				"jmag": poseSucc.Orientation().Quaternion().Jmag,
				"kmag": poseSucc.Orientation().Quaternion().Kmag,
			},
		}

		pose, componentRef, err := checkQuaternionFromClientAlgo(poseSucc, componentRefSucc, returnedExtSucc)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, spatial.PoseAlmostEqual(poseSucc, pose), test.ShouldBeTrue)
		test.That(t, componentRef, test.ShouldEqual, componentRefSucc)
	})

	t.Run("test failure due to quaternion not being given", func(t *testing.T) {

		poseSucc := spatial.NewPose(r3.Vector{X: 1, Y: 2, Z: 3}, &spatial.OrientationVector{Theta: math.Pi / 2, OX: 0, OY: 0, OZ: -1})
		componentRefSucc := "cam"
		returnedExtFail := map[string]interface{}{
			"badquat": map[string]interface{}{
				"real": poseSucc.Orientation().Quaternion().Real,
				"imag": poseSucc.Orientation().Quaternion().Imag,
				"jmag": poseSucc.Orientation().Quaternion().Jmag,
				"kmag": poseSucc.Orientation().Quaternion().Kmag,
			},
		}

		pose, componentRef, err := checkQuaternionFromClientAlgo(poseSucc, componentRefSucc, returnedExtFail)
		test.That(t, err.Error(), test.ShouldContainSubstring, "error getting SLAM position: quaternion not given")
		test.That(t, pose, test.ShouldBeNil)
		test.That(t, componentRef, test.ShouldBeEmpty)
	})

	t.Run("test failure due to invalid quaternion format", func(t *testing.T) {

		poseSucc := spatial.NewPose(r3.Vector{X: 1, Y: 2, Z: 3}, &spatial.OrientationVector{Theta: math.Pi / 2, OX: 0, OY: 0, OZ: -1})
		componentRefSucc := "cam"
		returnedExtFail := map[string]interface{}{
			"quat": map[string]interface{}{
				"realbad": poseSucc.Orientation().Quaternion().Real,
				"imagbad": poseSucc.Orientation().Quaternion().Imag,
				"jmagbad": poseSucc.Orientation().Quaternion().Jmag,
				"kmagbad": poseSucc.Orientation().Quaternion().Kmag,
			},
		}

		pose, componentRef, err := checkQuaternionFromClientAlgo(poseSucc, componentRefSucc, returnedExtFail)
		test.That(t, err.Error(), test.ShouldContainSubstring, "error getting SLAM position: quaternion given, but invalid format detected")
		test.That(t, pose, test.ShouldBeNil)
		test.That(t, componentRef, test.ShouldBeEmpty)
	})

}
