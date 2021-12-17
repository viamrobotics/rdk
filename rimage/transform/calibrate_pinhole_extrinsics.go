package transform

import (
	"fmt"
	"math"
	"math/rand"

	"go.viam.com/core/spatialmath"

	"github.com/edaniels/golog"
	"github.com/go-errors/errors"
	"github.com/golang/geo/r2"
	"github.com/golang/geo/r3"
	"gonum.org/v1/gonum/diff/fd"
	"gonum.org/v1/gonum/mat"
	"gonum.org/v1/gonum/optimize"
)

func RunPinholeExtrinsicCalibration(prob *optimize.Problem, logger golog.Logger) (spatialmath.Pose, error) {
	// optimization method
	method := &optimize.GradientDescent{
		StepSizer:         &optimize.FirstOrderStepSize{},
		GradStopThreshold: 1e-8,
	}
	// optimization settings
	settings := &optimize.Settings{
		GradientThreshold: 0,
		Converger: &optimize.FunctionConverge{
			Relative:   0.005,
			Absolute:   1e-8,
			Iterations: 100,
		},
	}
	// initial value for rotation euler angles(3) and translation(3)
	params := make([]float64, 6)
	for i := range params {
		params[i] = (rand.Float64() - 0.5) / 10.
	}
	// do the minimization
	res, err := optimize.Minimize(*prob, params, settings, method)
	logger.Infof("Function evaluation: %v", res.F)
	logger.Infof("Stats: %+v", res.Stats)
	logger.Infof("Status: %+v", res.Status)
	rotation := &spatialmath.EulerAngles{res.X[0], res.X[1], res.X[2]}
	translation := r3.Vector{res.X[3], res.X[4], res.X[5]}
	logger.Debugf("translation: %v", translation)
	logger.Debugf("rotation: %v", rotation.RotationMatrix())
	pose := spatialmath.NewPoseFromOrientation(translation, rotation)
	if err != nil {
		return pose, fmt.Errorf("%+v: %w", res.Status, err)
	}
	return pose, nil
}

func BuildExtrinsicOptProblem(depth, color *PinholeCameraIntrinsics, depthPoints []r3.Vector, colorPoints []r2.Point) (*optimize.Problem, error) {
	// check if the number of points in each image is the same
	if len(colorPoints) != len(depthPoints) {
		return nil, errors.Errorf("number of color points (%d) does not equal number of depth points (%d)", len(colorPoints), len(depthPoints))
	}
	// check if there are at least 4 points in each image
	if len(colorPoints) < 4 {
		return nil, errors.Errorf("need at least 4 points to calculate extrinsic matrix, only have %d", len(colorPoints))
	}
	for i, pt := range depthPoints {
		if pt.Z == 0.0 {
			return nil, errors.Errorf("point %d has a depth of 0. Zero depth is not allowed", i)
		}
	}
	depthPx, depthPy := depth.Ppx, depth.Ppy
	depthFx, depthFy := depth.Fx, depth.Fy
	colorPx, colorPy := color.Ppx, color.Ppy
	colorFx, colorFy := color.Fx, color.Fy
	N := len(colorPoints)
	m2mm := 1000.0 // all parameters should be around the same scale
	fcn := func(p []float64) float64 {
		// p[0] - roll-x, p[1] - pitch-y, p[2] - yaw-z
		rollRot := []float64{
			1, 0, 0,
			0, math.Cos(p[0]), math.Sin(p[0]),
			0, -math.Sin(p[0]), math.Cos(p[0]),
		}
		pitchRot := []float64{
			math.Cos(p[1]), 0, -math.Sin(p[1]),
			0, 1, 0,
			math.Sin(p[1]), 0, math.Cos(p[1]),
		}
		yawRot := []float64{
			math.Cos(p[2]), math.Sin(p[2]), 0,
			-math.Sin(p[2]), math.Cos(p[2]), 0,
			0, 0, 1,
		}
		translation := p[3:]
		mse := 0.0
		for i := 0; i < N; i++ {
			cPt := colorPoints[i]
			dPt := depthPoints[i]
			z := dPt.Z
			// 2D depth point to 3D
			x := z * ((dPt.X - depthPx) / depthFx)
			y := z * ((dPt.Y - depthPy) / depthFy)
			// use parameters to rigid transform points to color 3D
			x, y, z = x/m2mm, y/m2mm, z/m2mm
			// roll rollRot
			x = rollRot[0]*x + rollRot[1]*y + rollRot[2]*z
			y = rollRot[3]*x + rollRot[4]*y + rollRot[5]*z
			z = rollRot[6]*x + rollRot[7]*y + rollRot[8]*z
			// pitch rotation
			x = pitchRot[0]*x + pitchRot[1]*y + pitchRot[2]*z
			y = pitchRot[3]*x + pitchRot[4]*y + pitchRot[5]*z
			z = pitchRot[6]*x + pitchRot[7]*y + pitchRot[8]*z
			// yaw rotation
			x = yawRot[0]*x + yawRot[1]*y + yawRot[2]*z
			y = yawRot[3]*x + yawRot[4]*y + yawRot[5]*z
			z = yawRot[6]*x + yawRot[7]*y + yawRot[8]*z
			// translation
			x += translation[0]
			y += translation[1]
			z += translation[2]
			x, y, z = x*m2mm, y*m2mm, z*m2mm
			// color 3D to 2D point
			x = (x/z)*colorFx + colorPx
			y = (y/z)*colorFy + colorPy
			// compare the color image points to the projected points
			mse += math.Pow(x-cPt.X, 2)
			mse += math.Pow(y-cPt.Y, 2)
		}
		mse = mse / float64(N)
		return mse
	}
	grad := func(grad, x []float64) {
		fd.Gradient(grad, fcn, x, nil)
	}
	hess := func(h *mat.SymDense, x []float64) {
		fd.Hessian(h, fcn, x, nil)
	}
	problem := &optimize.Problem{Func: fcn, Grad: grad, Hess: hess}
	return problem, nil
}
