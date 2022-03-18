// Package main allows one to play around with a robotic arm.
package main

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"log"
	"math"
	"strconv"

	"github.com/edaniels/golog"
	"github.com/golang/geo/r3"
	"github.com/pkg/errors"
	"go.viam.com/utils"

	"go.viam.com/rdk/action"
	"go.viam.com/rdk/component/base"
	"go.viam.com/rdk/motionplan"
	frame "go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/robot"
	spatial "go.viam.com/rdk/spatialmath"
	webserver "go.viam.com/rdk/web/server"
)

var logger = golog.NewDevelopmentLogger("mobileRobotPlanning")

func main() {
	utils.ContextualMain(webserver.RunServer, logger)
}

func init() {
	action.RegisterAction("plan", func(ctx context.Context, r robot.Robot) {
		err := mobileRobotPlan(ctx, r)
		if err != nil {
			logger.Errorf("error planning: %s", err)
		}
	})
}

type Obstacle struct{ Center, Dims []float64 }

func mobileRobotPlan(ctx context.Context, r robot.Robot) error {
	names := base.NamesFromRobot(r)
	if len(names) != 1 {
		return errors.New("need at least one base")
	}
	mobileBase, err := base.FromRobot(r, names[0])
	if err != nil {
		return err
	}
	_ = mobileBase // TODO: implement functionality for mobilebase to follow plan

	// setup problem parameters
	start := []float64{-9, 9}
	goal := []float64{9, 9}
	robotDims := []float64{1, 1}
	limits := []frame.Limit{{Min: -10, Max: 10}, {Min: -10, Max: 10}}
	obstacles := []Obstacle{
		{Center: []float64{0, 6}, Dims: []float64{8, 8}},
		{Center: []float64{-9, -9}, Dims: []float64{2, 2}},
		{Center: []float64{9, -9}, Dims: []float64{2, 2}},
	}
	plan(ctx, start, goal, robotDims, limits, obstacles)
}

func plan(ctx context.Context, start, goal, robotDims []float64, limits []frame.Limit, obstacles []Obstacle) [][]frame.Input {
	// parse input
	startConfiguration := frame.FloatsToInputs(start)
	goalPose := spatial.PoseToProtobuf(spatial.NewPoseFromPoint(r3.Vector{X: goal[0], Y: goal[1], Z: 0}))
	robotGeometry, err := spatial.NewBoxCreator(r3.Vector{X: robotDims[0], Y: robotDims[1], Z: 1}, spatial.NewZeroPose())
	if err != nil {
		logger.Fatal(err.Error())
	}
	if limits == nil {
		limits = []frame.Limit{{Min: math.Inf(-1), Max: math.Inf(1)}, {Min: math.Inf(-1), Max: math.Inf(1)}}
	}
	obstacleGeometries := map[string]spatial.Geometry{}
	for i, obstacle := range obstacles {
		box, err := spatial.NewBox(spatial.NewPoseFromPoint(
			r3.Vector{obstacle.Center[0], obstacle.Center[1], 0}),
			r3.Vector{obstacle.Dims[0], obstacle.Dims[1], 1})
		if err != nil {
			logger.Fatal(err.Error())
		}
		obstacleGeometries[strconv.Itoa(i)] = box
	}

	// build model
	model, err := frame.NewMobileFrame("mobile-base", limits, robotGeometry)
	if err != nil {
		logger.Fatal(err.Error())
	}

	// setup planner
	cbert, err := motionplan.NewCBiRRTMotionPlanner(model, 1, logger)
	if err != nil {
		logger.Fatal(err.Error())
	}
	opt := motionplan.NewDefaultPlannerOptions()
	opt.AddConstraint("collision", motionplan.NewCollisionConstraintFromFrame(model, obstacleGeometries))

	// plan
	waypoints, err := cbert.Plan(ctx, goalPose, startConfiguration, opt)
	if err != nil {
		logger.Fatal(err.Error())
	}
}

func writeToFile(data interface{}) {
	bytes, err := json.Marshal(data)
	if err != nil {
		logger.Fatal(err.Error())
	}
	if err := ioutil.WriteFile("samples/mobileRobotPlanning/planOutput.json", bytes, 0644); err != nil {
		log.Fatal(err.Error())
	}
}
