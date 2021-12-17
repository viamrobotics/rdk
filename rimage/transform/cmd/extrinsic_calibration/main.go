// Given at least 4 corresponding points, and the intrinsic matrices of both cameras, computes
// the rigid transform (rotation + translation) that would be the extrinsic transformation from camera 1 to camera 2.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"os"

	"go.viam.com/core/rimage/transform"
	"go.viam.com/core/spatialmath"
	"go.viam.com/utils"

	"github.com/edaniels/golog"
	"github.com/go-errors/errors"
	"github.com/golang/geo/r2"
	"github.com/golang/geo/r3"
)

type CalibrationConfig struct {
	ColorPoints     []r2.Point                        `json:"color_points"`
	DepthPoints     []r3.Vector                       `json:"depth_points"`
	ColorIntrinsics transform.PinholeCameraIntrinsics `json:"color_intrinsics"`
	DepthIntrinsics transform.PinholeCameraIntrinsics `json:"depth_intrinsics"`
}

func main() {
	confPtr := flag.String("conf", "", "path of configuration for extrinsic parameter finding")
	flag.Parse()
	logger := golog.NewLogger("extrinsic_calibration")
	calibrate(*confPtr, logger)
	os.Exit(0)
}

func calibrate(conf string, logger golog.Logger) {
	cfg, err := readConfig(conf)
	if err != nil {
		logger.Fatal(err)
	}
	// set up the optimization problem
	problem, err := transform.BuildExtrinsicOptProblem(&cfg.DepthIntrinsics, &cfg.ColorIntrinsics, cfg.DepthPoints, cfg.ColorPoints)
	if err != nil {
		logger.Fatal(err)
	}
	// solve the problem
	pose, err := transform.RunPinholeExtrinsicCalibration(problem, logger)
	// print result to output stream
	fmt.Printf("\nrotation:\n%v\ntranslation:\n%.3f\n", printRot(pose.Orientation()), pose.Point())
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
	return conf, nil
}
