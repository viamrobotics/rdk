// Given at least 4 corresponding points, and the intrinsic matrices of both cameras, computes
// the rigid transform (rotation + translation) that would be the extrinsic transformation from camera 1 to camera 2.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"math"
	"math/rand"
	"os"

	"go.viam.com/core/rimage/transform"
	"go.viam.com/core/spatialmath"
	"go.viam.com/utils"

	"github.com/edaniels/golog"
	"github.com/go-errors/errors"
	"github.com/golang/geo/r2"
	"github.com/golang/geo/r3"
	"gonum.org/v1/gonum/diff/fd"
	"gonum.org/v1/gonum/mat"
	"gonum.org/v1/gonum/optimize"
)

type CalibrationConfig struct {
	ColorPoints     []r2.Point                        `json:"color_points"`
	DepthPoints     []r3.Vector                       `json:"depth_points"`
	ColorIntrinsics transform.PinholeCameraIntrinsics `json:"color_intrinsics"`
	DepthIntrinsics transform.PinholeCameraIntrinsics `json:"depth_intrinsics"`
}

var logger = golog.NewLogger("extrinsic_calibration")

func main() {
	confPtr := flag.String("conf", "", "path of configuration for extrinsic parameter finding")
	flag.Parse()
	// load the inputs from the config file
	cfg, err := readConfig(*confPtr)
	if err != nil {
		logger.Fatal(err)
	}
	// set up the optimization problem
	problem, err := createProblem(cfg)
	if err != nil {
		logger.Fatal(err)
	}
	// solve the problem
	method := &optimize.GradientDescent{}
	//method := &optimize.Newton{}
	params := make([]float64, 6) // initial value for rotation euler angles(3) and translation(3)
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
	rotation := &spatialmath.EulerAngles{res.X[0], res.X[1], res.X[2]}
	translation := res.X[3:]
	final := rotation.RotationMatrix()
	fmt.Printf(" rotation:\n%v\n%v\n%v\n translation:\n%v\n", final.Row(0), final.Row(1), final.Row(2), translation)
}

func readConfig(cfgPath string) (*CalibrationConfig, error) {
	f, err := os.Open(cfgPath)
	if err != nil {
		return nil, errors.Errorf("path=%q: %w", cfgPath, err)
	}
	defer utils.UncheckedErrorFunc(f.Close)

	byteJSON, err := ioutil.ReadAll(f)
	if err != nil {
		return nil, err
	}

	conf := &CalibrationConfig{}
	err = json.Unmarshal(byteJSON, conf)
	if err != nil {
		return nil, errors.Errorf("error parsing byte array - %w", err)
	}
	// check if the number of points in each image is the same
	if len(conf.ColorPoints) != len(conf.DepthPoints) {
		return nil, errors.Errorf("number of color points (%d) does not equal number of depth points (%d)", len(conf.ColorPoints), len(conf.DepthPoints))
	}
	// check if there are at least 4 points in each image
	if len(conf.ColorPoints) < 4 {
		return nil, errors.Errorf("need at least 4 points to calculate extrinsic matrix, only have %d", len(conf.ColorPoints))
	}
	for i, pt := range conf.DepthPoints {
		if pt.Z == 0.0 {
			return nil, errors.Errorf("point %d has a depth of 0. Zero depth is not allowed", i)
		}
	}
	return conf, nil
}

func createProblem(cfg *CalibrationConfig) (optimize.Problem, error) {
	depthPx, depthPy := cfg.DepthIntrinsics.Ppx, cfg.DepthIntrinsics.Ppy
	depthFx, depthFy := cfg.DepthIntrinsics.Fx, cfg.DepthIntrinsics.Fy
	colorPx, colorPy := cfg.ColorIntrinsics.Ppx, cfg.ColorIntrinsics.Ppy
	colorFx, colorFy := cfg.ColorIntrinsics.Fx, cfg.ColorIntrinsics.Fy
	m2mm := 1000.0
	fcn := func(p []float64) float64 {
		// p[0] - roll(3), p[1] - pitch(2), p[2] - yaw(1)
		rotation := []float64{
			math.Cos(p[2]) * math.Cos(p[1]),
			math.Cos(p[2])*math.Sin(p[1])*math.Sin(p[0]) - math.Cos(p[0])*math.Cos(p[2]),
			math.Sin(p[2])*math.Sin(p[0]) + math.Cos(p[2])*math.Cos(p[0])*math.Sin(p[1]),
			math.Cos(p[1]) * math.Sin(p[2]),
			math.Cos(p[2])*math.Cos(p[0]) + math.Sin(p[2])*math.Sin(p[1])*math.Sin(p[0]),
			math.Cos(p[0])*math.Sin(p[2])*math.Sin(p[1]) - math.Cos(p[2])*math.Sin(p[0]),
			-math.Sin(p[1]),
			math.Cos(p[1]) * math.Sin(p[0]),
			math.Cos(p[1]) * math.Cos(p[0]),
		}
		translation := p[3:]
		mse := 0.0
		for i := range cfg.ColorPoints {
			cPt := cfg.ColorPoints[i]
			dPt := cfg.DepthPoints[i]
			z := dPt.Z
			// 2D depth point to 3D
			x := z * (dPt.X - depthPx) / depthFx
			y := z * (dPt.Y - depthPy) / depthFy
			// use parameters to rigid transform points to color 3D
			x, y, z = x/m2mm, y/m2mm, z/m2mm
			x = rotation[0]*x + rotation[1]*y + rotation[2]*z + translation[0]
			y = rotation[3]*x + rotation[4]*y + rotation[5]*z + translation[1]
			z = rotation[6]*x + rotation[7]*y + rotation[8]*z + translation[2]
			x, y, z = x*m2mm, y*m2mm, z*m2mm
			// color 3D to 2D point
			x = (x/z)*colorFx + colorPx
			y = (y/z)*colorFy + colorPy
			// compare the color image points to the projected points
			mse += math.Pow(x-cPt.X, 2)
			mse += math.Pow(y-cPt.Y, 2)
		}
		mse = mse / float64(len(cfg.ColorPoints))
		// add constraints of an orthogonal matrix to the rotation parameters
		return mse
	}
	grad := func(grad, x []float64) {
		fd.Gradient(grad, fcn, x, nil)
	}

	hess := func(h *mat.SymDense, x []float64) {
		fd.Hessian(h, fcn, x, nil)
	}
	problem := optimize.Problem{Func: fcn, Grad: grad, Hess: hess}
	return problem, nil
}
