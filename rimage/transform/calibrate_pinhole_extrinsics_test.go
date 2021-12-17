package transform

import (
	"math"
	"math/rand"
	"testing"

	"go.viam.com/core/spatialmath"
	"go.viam.com/core/utils"
	"go.viam.com/test"

	"github.com/edaniels/golog"
	"github.com/golang/geo/r2"
	"github.com/golang/geo/r3"
)

func TestExtrinsicCalibration(t *testing.T) {
	logger := golog.NewTestLogger(t)
	// get a file with known extrinsic parameters and make expected pose
	cam, err := NewDepthColorIntrinsicsExtrinsicsFromJSONFile(utils.ResolveFile("robots/configs/intel515_parameters.json"))
	test.That(t, err, test.ShouldBeNil)
	rotation, err := spatialmath.NewRotationMatrix(cam.ExtrinsicD2C.RotationMatrix)
	test.That(t, err, test.ShouldBeNil)
	translation := cam.ExtrinsicD2C.TranslationVector

	// create many points from a known extrinsic file
	n := 1000
	depthH, depthW := float64(cam.DepthCamera.Height), float64(cam.DepthCamera.Width)
	colorPoints := make([]r2.Point, n)
	depthPoints := make([]r3.Vector, n)
	for i := 0; i < n; i++ {
		dx := math.Round(rand.Float64() * depthW)
		dy := math.Round(rand.Float64() * depthH)
		dz := math.Round(rand.Float64()*2450.) + 50.0 // always want at least 50 mm distance
		depthPoints[i] = r3.Vector{dx, dy, dz}
		cx, cy, _ := cam.DepthPixelToColorPixel(dx, dy, dz)
		colorPoints[i] = r2.Point{cx, cy}
	}

	prob, err := BuildExtrinsicOptProblem(cam.DepthCamera, cam.ColorCamera, depthPoints, colorPoints)
	test.That(t, err, test.ShouldBeNil)
	pose, err := RunPinholeExtrinsicCalibration(prob, logger)
	test.That(t, err, test.ShouldBeNil)
	point := pose.Point()
	orientation := pose.Orientation()

	test.That(t, point.X, test.ShouldAlmostEqual, translation[0])
	test.That(t, point.Y, test.ShouldAlmostEqual, translation[1])
	test.That(t, point.Z, test.ShouldAlmostEqual, translation[2])
	test.That(t, orientation, test.ShouldResemble, rotation)
}
