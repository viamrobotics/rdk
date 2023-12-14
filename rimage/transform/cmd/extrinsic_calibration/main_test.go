package main

import (
	"encoding/json"
	"math"
	"math/rand"
	"os"
	"testing"

	"github.com/golang/geo/r2"
	"github.com/golang/geo/r3"
	"go.viam.com/test"
	"gonum.org/v1/gonum/optimize"

	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/rimage/transform"
	"go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/utils"
)

type mockPose struct{}

func (m mockPose) Point() r3.Vector {
	return r3.Vector{}
}

func (m mockPose) Orientation() spatialmath.Orientation {
	return spatialmath.NewOrientationVector()
}

func TestMainCalibrate(t *testing.T) {
	outDir := t.TempDir()
	logger := logging.NewTestLogger(t)

	// get a file with known extrinsic parameters
	camera, err := transform.NewDepthColorIntrinsicsExtrinsicsFromJSONFile(utils.ResolveFile("rimage/transform/data/intel515_parameters.json"))
	test.That(t, err, test.ShouldBeNil)

	// create many points from a known extrinsic file
	calibConfig := createInputConfig(camera, 100)
	// writes bytes to temporary file
	b, err := json.MarshalIndent(calibConfig, "", " ")
	test.That(t, err, test.ShouldBeNil)
	err = os.WriteFile(outDir+"/test.json", b, 0o644)
	test.That(t, err, test.ShouldBeNil)

	mockCalibrationFn := func(_ *optimize.Problem, _ logging.Logger) (spatialmath.Pose, error) {
		return mockPose{}, nil
	}

	// read from temp file and process
	calibrate(outDir+"/test.json", logger, mockCalibrationFn)
}

func createInputConfig(c *transform.DepthColorIntrinsicsExtrinsics, n int) *transform.ExtrinsicCalibrationConfig {
	depthH, depthW := float64(c.DepthCamera.Height), float64(c.DepthCamera.Width)
	colorPoints := make([]r2.Point, n)
	depthPoints := make([]r3.Vector, n)
	for i := 0; i < n; i++ {
		dx := math.Round(rand.Float64() * depthW)
		dy := math.Round(rand.Float64() * depthH)
		dz := math.Round(rand.Float64()*2450.) + 50.0 // always want at least 50 mm distance
		depthPoints[i] = r3.Vector{dx, dy, dz}
		cx, cy, _ := c.DepthPixelToColorPixel(dx, dy, dz)
		colorPoints[i] = r2.Point{cx, cy}
	}
	conf := &transform.ExtrinsicCalibrationConfig{
		ColorPoints:     colorPoints,
		DepthPoints:     depthPoints,
		ColorIntrinsics: c.ColorCamera,
		DepthIntrinsics: c.DepthCamera,
	}
	return conf
}
