package utils

import (
	"math"
	"sort"
	"strings"
	"testing"

	"github.com/golang/geo/r3"
	"go.viam.com/test"

	"go.viam.com/rdk/spatialmath"
)

func TestDictToString(t *testing.T) {
	t.Run("Convert dictionary to a string", func(t *testing.T) {
		configParamsDict := map[string]string{
			"min_range": "0.3",
			"max_range": "12",
			"debug":     "false",
			"mode":      "2d",
		}

		expectedConfigParamsString := "{debug=false,mode=2d,min_range=0.3,max_range=12}"
		actualConfigParamsString := DictToString(configParamsDict)
		test.That(t, actualConfigParamsString[0], test.ShouldEqual, expectedConfigParamsString[0])

		expectedLastLetter := expectedConfigParamsString[len(expectedConfigParamsString)-1:]
		actualLastLetter := actualConfigParamsString[len(actualConfigParamsString)-1:]
		test.That(t, actualLastLetter, test.ShouldEqual, expectedLastLetter)

		expectedContents := strings.Split(expectedConfigParamsString[1:len(expectedConfigParamsString)-1], ",")
		actualContents := strings.Split(actualConfigParamsString[1:len(actualConfigParamsString)-1], ",")
		sort.Strings(actualContents)
		sort.Strings(expectedContents)
		test.That(t, actualContents, test.ShouldResemble, expectedContents)
	})
}

func TestBuiltinQuaternion(t *testing.T) {
	poseSucc := spatialmath.NewPose(r3.Vector{X: 1, Y: 2, Z: 3}, &spatialmath.OrientationVector{Theta: math.Pi / 2, OX: 0, OY: 0, OZ: -1})
	componentRefSucc := "cam"
	t.Run("test successful quaternion from internal server", func(t *testing.T) {
		returnedExtSucc := map[string]interface{}{
			"quat": map[string]interface{}{
				"real": poseSucc.Orientation().Quaternion().Real,
				"imag": poseSucc.Orientation().Quaternion().Imag,
				"jmag": poseSucc.Orientation().Quaternion().Jmag,
				"kmag": poseSucc.Orientation().Quaternion().Kmag,
			},
		}

		pose, componentRef, err := CheckQuaternionFromClientAlgo(poseSucc, componentRefSucc, returnedExtSucc)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, spatialmath.PoseAlmostEqual(poseSucc, pose), test.ShouldBeTrue)
		test.That(t, componentRef, test.ShouldEqual, componentRefSucc)
	})

	t.Run("test failure due to quaternion not being given", func(t *testing.T) {
		returnedExtFail := map[string]interface{}{
			"badquat": map[string]interface{}{
				"real": poseSucc.Orientation().Quaternion().Real,
				"imag": poseSucc.Orientation().Quaternion().Imag,
				"jmag": poseSucc.Orientation().Quaternion().Jmag,
				"kmag": poseSucc.Orientation().Quaternion().Kmag,
			},
		}

		pose, componentRef, err := CheckQuaternionFromClientAlgo(poseSucc, componentRefSucc, returnedExtFail)
		test.That(t, err.Error(), test.ShouldContainSubstring, "error getting SLAM position: quaternion not given")
		test.That(t, pose, test.ShouldBeNil)
		test.That(t, componentRef, test.ShouldBeEmpty)
	})

	t.Run("test failure due to invalid quaternion format", func(t *testing.T) {
		returnedExtFail := map[string]interface{}{
			"quat": map[string]interface{}{
				"realbad": poseSucc.Orientation().Quaternion().Real,
				"imagbad": poseSucc.Orientation().Quaternion().Imag,
				"jmagbad": poseSucc.Orientation().Quaternion().Jmag,
				"kmagbad": poseSucc.Orientation().Quaternion().Kmag,
			},
		}

		pose, componentRef, err := CheckQuaternionFromClientAlgo(poseSucc, componentRefSucc, returnedExtFail)
		test.That(t, err.Error(), test.ShouldContainSubstring, "error getting SLAM position: quaternion given, but invalid format detected")
		test.That(t, pose, test.ShouldBeNil)
		test.That(t, componentRef, test.ShouldBeEmpty)
	})
}
