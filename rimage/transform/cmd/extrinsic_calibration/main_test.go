package main

import (
	"encoding/json"
	"io/ioutil"
	"math"
	"math/rand"
	"testing"

	"go.viam.com/core/rimage/transform"
	"go.viam.com/core/utils"
	"go.viam.com/test"
	"go.viam.com/utils/testutils"

	"github.com/edaniels/golog"
	"github.com/golang/geo/r2"
	"github.com/golang/geo/r3"
)

func TestMainCalibrate(t *testing.T) {
	outDir := testutils.TempDirT(t, "", "transform_cmd_extrinsic_calibration")
	logger := golog.NewTestLogger(t)

	// get a file with known extrinsic parameters
	camera, err := transform.NewDepthColorIntrinsicsExtrinsicsFromJSONFile(utils.ResolveFile("robots/configs/intel515_parameters.json"))
	test.That(t, err, test.ShouldBeNil)

	// create many points from a known extrinsic file
	calibConfig := createInputConfig(camera, 100)
	// writes bytes to temporary file
	b, err := json.MarshalIndent(calibConfig, "", " ")
	test.That(t, err, test.ShouldBeNil)
	err = ioutil.WriteFile(outDir+"/test.json", b, 0644)
	test.That(t, err, test.ShouldBeNil)

	// read from temp file and process
	calibrate(outDir+"/test.json", logger)
}

func createInputConfig(c *transform.DepthColorIntrinsicsExtrinsics, n int) *CalibrationConfig {
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
	conf := &CalibrationConfig{
		ColorPoints:     colorPoints,
		DepthPoints:     depthPoints,
		ColorIntrinsics: c.ColorCamera,
		DepthIntrinsics: c.DepthCamera,
	}
	return conf
}
