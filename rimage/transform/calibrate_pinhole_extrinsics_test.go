package transform

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"testing"

	"github.com/edaniels/golog"
	"go.viam.com/test"

	"go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/utils"
)

func TestExtrinsicCalibration(t *testing.T) {
	logger := golog.NewTestLogger(t)
	// get a file with known extrinsic parameters and make expected pose
	cam, err := NewDepthColorIntrinsicsExtrinsicsFromJSONFile(utils.ResolveFile("robots/configs/intel515_parameters.json"))
	test.That(t, err, test.ShouldBeNil)
	expRotation, err := spatialmath.NewRotationMatrix(cam.ExtrinsicD2C.RotationMatrix)
	test.That(t, err, test.ShouldBeNil)
	expTranslation := cam.ExtrinsicD2C.TranslationVector

	// get points and intrinsics from test file
	jsonFile, err := os.Open(utils.ResolveFile("rimage/transform/example_extrinsic_calib.json"))
	test.That(t, err, test.ShouldBeNil)
	defer jsonFile.Close()

	byteValue, err := io.ReadAll(jsonFile)
	test.That(t, err, test.ShouldBeNil)

	extConf := &ExtrinsicCalibrationConfig{}
	err = json.Unmarshal(byteValue, extConf)
	test.That(t, err, test.ShouldBeNil)

	// create the optimization problem
	prob, err := BuildExtrinsicOptProblem(extConf)
	test.That(t, err, test.ShouldBeNil)
	pose, err := RunPinholeExtrinsicCalibration(prob, logger)
	test.That(t, err, test.ShouldBeNil)
	translation := pose.Point()
	rotation := pose.Orientation()

	// only test to 3 digits for found translation and rotation
	test.That(t, fmt.Sprintf("%.3f", translation.X), test.ShouldEqual, fmt.Sprintf("%.3f", expTranslation[0]))
	test.That(t, fmt.Sprintf("%.3f", translation.Y), test.ShouldEqual, fmt.Sprintf("%.3f", expTranslation[1]))
	test.That(t, fmt.Sprintf("%.3f", translation.Z), test.ShouldEqual, fmt.Sprintf("%.3f", expTranslation[2]))
	q, expq := rotation.Quaternion(), expRotation.Quaternion()
	test.That(t, fmt.Sprintf("%.3f", q.Real), test.ShouldEqual, fmt.Sprintf("%.3f", expq.Real))
	test.That(t, fmt.Sprintf("%.3f", q.Imag), test.ShouldEqual, fmt.Sprintf("%.3f", expq.Imag))
	test.That(t, fmt.Sprintf("%.3f", q.Jmag), test.ShouldEqual, fmt.Sprintf("%.3f", expq.Jmag))
	test.That(t, fmt.Sprintf("%.3f", q.Kmag), test.ShouldEqual, fmt.Sprintf("%.3f", expq.Kmag))
}
