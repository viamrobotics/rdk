package transform

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"testing"

	"go.viam.com/test"

	"go.viam.com/core/spatialmath"
	"go.viam.com/core/utils"

	"github.com/edaniels/golog"
	"github.com/golang/geo/r2"
	"github.com/golang/geo/r3"
)

func TestExtrinsicCalibration(t *testing.T) {
	logger := golog.NewTestLogger(t)
	// get a file with known extrinsic parameters and make expected pose
	cam, err := NewDepthColorIntrinsicsExtrinsicsFromJSONFile(utils.ResolveFile("robots/configs/intel515_parameters.json"))
	test.That(t, err, test.ShouldBeNil)
	expRotation, err := spatialmath.NewRotationMatrix(cam.ExtrinsicD2C.RotationMatrix)
	test.That(t, err, test.ShouldBeNil)
	expTranslation := cam.ExtrinsicD2C.TranslationVector

	// get points and intrinsics from known file
	depthIntrin, colorIntrin, depthPoints, colorPoints := loadParameters(t, utils.ResolveFile("rimage/transform/example_extrinsic_calib.json"))

	// create the optimization problem
	prob, err := BuildExtrinsicOptProblem(depthIntrin, colorIntrin, depthPoints, colorPoints)
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

func loadParameters(t *testing.T, filePath string) (*PinholeCameraIntrinsics, *PinholeCameraIntrinsics, []r3.Vector, []r2.Point) {
	jsonFile, err := os.Open(filePath)
	test.That(t, err, test.ShouldBeNil)
	defer jsonFile.Close()

	byteValue, err := ioutil.ReadAll(jsonFile)
	test.That(t, err, test.ShouldBeNil)

	temp := struct {
		ColorPoints []r2.Point              `json:"color_points"`
		DepthPoints []r3.Vector             `json:"depth_points"`
		Color       PinholeCameraIntrinsics `json:"color_intrinsics"`
		Depth       PinholeCameraIntrinsics `json:"depth_intrinsics"`
	}{}
	err = json.Unmarshal(byteValue, &temp)
	test.That(t, err, test.ShouldBeNil)
	return &temp.Depth, &temp.Color, temp.DepthPoints, temp.ColorPoints
}
