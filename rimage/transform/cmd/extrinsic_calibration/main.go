// Given at least 4 corresponding points, and the intrinsic matrices of both cameras, computes
// the rigid transform (rotation + translation) that would be the extrinsic transformation from camera 1 to camera 2.
package main

import (
	"flag"
	"fmt"
	"image"
	"os"

	"go.viam.com/core/rimage"
	"go.viam.com/core/rimage/transform"

	"github.com/golang/geo/r3"
	"gonum.org/v1/gonum/mat"
	"gonum.org/v1/gonum/optimize"
)

type CalibrationConfig struct {
	ColorPoints     []image.Point            `json:"color_points"`
	DepthPoints     []r3.Vector              `json:"depth_points"`
	ColorIntrinsics *PinholeCameraIntrinsics `json:"color_intrinsics"`
	DepthIntrinsics *PinholeCameraIntrinsics `json:"depth_intrinsics"`
}

var logger = golog.NewDevelopmentLogger("extrinsic_calibration")

func main() {
	confPath := flag.String("conf", "", "path of configuration for extrinsic parameter finding")
	// load the inputs from the config file
	cfg, err := readConfig(confPath)
	if err != nil {
		logger.Fatal(err)
	}
	// set up the optimization problem
	problem, err := createProblem(cfg)
	if err != nil {
		logger.Fatal(err)
	}
	// solve the problem
	method := &optimize.Newton{}
	params := make([]float64, 12) // initial value for rotation(9) and translation(3)
	for i := range params {
		params[i] = rand.Float64() - 0.5
	}
	res, err := optimize.Minimize(problem, params, nil, method)
	if err != nil {
		logger.Fatal(err)
	}
	fmt.Printf("optimization status code: %v\n", res.Status)
	fmt.Printf("optimization stats: %v\n", res.Stats)
	fmt.Printf("function value at end: %v\n", res.F)
	// return depth-to-color rotations and translation
	final := res.X
	fmt.Printf(" rotation:\n%v\n%v\n%v\n translation:\n%v\n", final[:3], final[3:6], final[6:9], final[9:])
}

func readConfig(cfgPath string) (*CalibrationConfig, error) {
	// check if the number of poitns in each image is the same
	// check if there are at least 4 points in each image
}

func createProblem(cfg *CalibrationConfig) (*optimize.Problem, error) {
	fcn := func(p []float64) float64 {
		ext := transform.Extrinsics{p[:9], p[9:]}
		camera := transform.DepthColorIntrinsicsExtrinsics{cfg.ColorIntrinsics, cfg.DepthIntrinsics, ext}
		res := 0.0
		for i := range cfg.ColorPoints {
			cPt := cfg.ColorPoints[i]
			dPt := cfg.DepthPoints[i]
			x, y, z := camera.DepthPixelToColorPixel(dPt.X, dPt.Y, dPt.Z)
			// compare the color image points to the projected points
			res += math.Pow(x-cPt.X, 2)
			res += math.Pow(y-cPt.Y, 2)
		}
		// add constraints of an orthogonal matrix to the rotation parameters
		return res
	}
	problem := optimize.Problem{Func: fcn, Grad: grad, Hess: hess}
	return &problem, nil
}
