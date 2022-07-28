// Package main allows one to play around with planning for a omnidirectional 2D mobile base
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math/rand"
	"strconv"

	"github.com/edaniels/golog"
	"github.com/golang/geo/r3"
	"github.com/pkg/errors"
	"go.uber.org/zap"

	"go.viam.com/rdk/motionplan"
	frame "go.viam.com/rdk/referenceframe"
	spatial "go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/utils"
)

var logger, err = zap.Config{
	Level:             zap.NewAtomicLevelAt(zap.FatalLevel),
	Encoding:          "console",
	DisableStacktrace: true,
}.Build()

func main() {
	config, err := parseJSONFile(utils.ResolveFile("samples/mobileRobotPlanning/planConfig.json"))
	if err != nil {
		logger.Fatal(err.Error())
	}

	test("CBiRRT", config)
	test("RRT", config)
}

type obstacle struct {
	Center []float64 `json:"center"`
	Dims   []float64 `json:"dims"`
}

type mobileRobotPlanConfig struct {
	// number of tests to run
	NumTests int `json:"tests"`

	// planning conditions
	Start []float64 `json:"start"`
	Goal  []float64 `json:"goal"`

	// robot params
	RobotDims []float64 `json:"robot-dims"`

	// map definition
	Xlim      []float64  `json:"xlim"`
	YLim      []float64  `json:"ylim"`
	Obstacles []obstacle `json:"obstacles"`
}

type plannerConstructor func(frame frame.Frame, nCPU int, seed *rand.Rand, logger golog.Logger) (motionplan.MotionPlanner, error)

func test(planner string, config *mobileRobotPlanConfig) {
	fmt.Println(planner)
	total := 0.
	var waypoints [][]frame.Input
	for i := 0; i < config.NumTests; i++ {
		switch planner {
		case "CBiRRT":
			waypoints, err = plan(context.Background(), motionplan.NewCBiRRTMotionPlanner, config, i)
		case "RRT":
			waypoints, err = plan(context.Background(), motionplan.NewRRTConnectMotionPlanner, config, i)
		default:
			logger.Fatal("planner " + planner + " not supported")
		}
		if err != nil {
			logger.Fatal(err.Error())
		}
		score := evaluate(waypoints)
		fmt.Println("Test ", i, ":\t", score)
		total += score
	}
	fmt.Print("Average:\t", total/float64(config.NumTests), "\n\n")

	// write output
	if err := writeJSONFile(utils.ResolveFile("samples/mobileRobotPlanning/"+planner+"Output.test"), waypoints); err != nil {
		logger.Fatal(err.Error())
	}
}

func plan(ctx context.Context, planner plannerConstructor, config *mobileRobotPlanConfig, seed int) ([][]frame.Input, error) {
	// parse input
	start := frame.FloatsToInputs(config.Start)
	goal := spatial.PoseToProtobuf(spatial.NewPoseFromPoint(r3.Vector{X: config.Goal[0], Y: config.Goal[1], Z: 0}))
	robotGeometry, err := spatial.NewBoxCreator(r3.Vector{X: config.RobotDims[0], Y: config.RobotDims[1], Z: 1}, spatial.NewZeroPose())
	if err != nil {
		return nil, err
	}
	limits := []frame.Limit{{Min: config.Xlim[0], Max: config.Xlim[1]}, {Min: config.YLim[0], Max: config.YLim[1]}}
	// TODO(rb) add logic to parse limit input to check for infinite limits
	// limits = []frame.Limit{{Min: math.Inf(-1), Max: math.Inf(1)}, {Min: math.Inf(-1), Max: math.Inf(1)}}
	obstacleGeometries := map[string]spatial.Geometry{}
	for i, o := range config.Obstacles {
		box, err := spatial.NewBox(spatial.NewPoseFromPoint(
			r3.Vector{X: o.Center[0], Y: o.Center[1], Z: 0}),
			r3.Vector{X: o.Dims[0], Y: o.Dims[1], Z: 1})
		if err != nil {
			return nil, err
		}
		obstacleGeometries[strconv.Itoa(i)] = box
	}

	// build model
	model, err := frame.NewMobile2DFrame("mobile-base", limits, robotGeometry)
	if err != nil {
		return nil, err
	}

	// setup planner
	mp, err := planner(model, 1, rand.New(rand.NewSource(1)), logger.Sugar())
	if err != nil {
		return nil, err
	}
	opt := motionplan.NewDefaultPlannerOptions()
	opt.AddConstraint("collision", motionplan.NewCollisionConstraint(model, obstacleGeometries, map[string]spatial.Geometry{}))

	// plan
	waypoints, err := mp.Plan(ctx, goal, start, opt)
	if err != nil {
		return nil, err
	}
	return waypoints, nil
}

func evaluate(waypoints [][]frame.Input) float64 {
	distance := 0.
	for i := 0; i < len(waypoints)-1; i++ {
		distance += motionplan.L2Distance(frame.InputsToFloats(waypoints[i]), frame.InputsToFloats(waypoints[i+1]))
	}
	return distance
}

func parseJSONFile(filename string) (*mobileRobotPlanConfig, error) {
	jsonData, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, errors.Wrap(err, "failed to read json file")
	}
	config := &mobileRobotPlanConfig{}
	if len(jsonData) == 0 {
		return nil, errors.New("no model information")
	}
	err = json.Unmarshal(jsonData, config)
	if err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal json file")
	}

	// assert correctness of json
	wrongDimsError := errors.New("need array of floats to have exactly 2 elements")
	if len(config.Start) != 2 {
		return nil, errors.Wrap(wrongDimsError, "config error in start field")
	}
	if len(config.Goal) != 2 {
		return nil, errors.Wrap(wrongDimsError, "config error in start field")
	}
	if len(config.Xlim) != 2 {
		return nil, errors.Wrap(wrongDimsError, "config error in xlim field")
	}
	if len(config.Xlim) != 2 {
		return nil, errors.Wrap(wrongDimsError, "config error in ylim field")
	}
	if len(config.RobotDims) != 2 {
		return nil, errors.Wrap(wrongDimsError, "config error in robot-dims field")
	}
	for _, o := range config.Obstacles {
		if len(o.Center) != 2 {
			return nil, errors.Wrap(wrongDimsError, "config error in obstacles.center field")
		}
		if len(o.Dims) != 2 {
			return nil, errors.Wrap(wrongDimsError, "config error in obstacles.dims field")
		}
	}
	return config, nil
}

func writeJSONFile(filename string, data interface{}) error {
	bytes, err := json.MarshalIndent(data, "", " ")
	if err != nil {
		return err
	}
	if err := ioutil.WriteFile(filename, bytes, 0o644); err != nil {
		return err
	}
	return nil
}
