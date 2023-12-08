// Given at least 4 corresponding points, and the intrinsic matrices of both cameras, computes
// the rigid transform (rotation + translation) that would be the extrinsic transformation
// from camera 1 to camera 2.
// rimage/transform/data/example_extrinsic_calib.json has an example input file.
// $./extrinsic_calibration -conf=/path/to/input/file
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"

	"gonum.org/v1/gonum/optimize"

	"github.com/pkg/errors"
	"go.viam.com/utils"
	"gonum.org/v1/gonum/optimize"

	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/rimage/transform"
	"go.viam.com/rdk/spatialmath"
)

func main() {
	confPtr := flag.String("conf", "", "path of configuration for extrinsic parameter finding")
	flag.Parse()
	logger := logging.NewLogger("extrinsic_calibration")
	calibrate(*confPtr, logger, transform.RunPinholeExtrinsicCalibration)
	os.Exit(0)
}

type calibrationFn = func(prob *optimize.Problem, logger logging.Logger) (spatialmath.Pose, error)

func calibrate(conf string, logger logging.Logger, fn calibrationFn) {
	cfg, err := readConfig(conf)
	if err != nil {
		logger.Fatal(err)
	}
	// set up the optimization problem
	problem, err := transform.BuildExtrinsicOptProblem(cfg)
	if err != nil {
		logger.Fatal(err)
	}
	// solve the problem
	pose, err := fn(problem, logger)
	// print result to output stream
	logger.Infof("\nrotation:\n%v\ntranslation:\n%.3f\n", printRot(pose.Orientation()), pose.Point())
	if err != nil {
		logger.Fatal(err)
	}
}

func printRot(o spatialmath.Orientation) string {
	final := o.RotationMatrix()
	r1, r2, r3 := final.Row(0), final.Row(1), final.Row(2)
	w1 := fmt.Sprintf("⸢ %.3f %.3f %.3f ⸣\n", r1.X, r1.Y, r1.Z)
	w2 := fmt.Sprintf("| %.3f %.3f %.3f |\n", r2.X, r2.Y, r2.Z)
	w3 := fmt.Sprintf("⸤ %.3f %.3f %.3f ⸥", r3.X, r3.Y, r3.Z)
	return w1 + w2 + w3
}

func readConfig(cfgPath string) (*transform.ExtrinsicCalibrationConfig, error) {
	f, err := os.Open(cfgPath) //nolint:gosec
	if err != nil {
		return nil, errors.Wrap(err, fmt.Sprintf("path=%q", cfgPath))
	}
	defer utils.UncheckedErrorFunc(f.Close)

	byteJSON, err := io.ReadAll(f)
	if err != nil {
		return nil, err
	}

	conf := &transform.ExtrinsicCalibrationConfig{}
	err = json.Unmarshal(byteJSON, conf)
	if err != nil {
		return nil, errors.Wrap(err, "error parsing byte array ")
	}
	return conf, nil
}
