// Package agile is an agile robot playground.
package main

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"strconv"

	"github.com/edaniels/golog"
	"github.com/golang/geo/r3"
	"github.com/pkg/errors"
	"go.viam.com/utils"
	"go.viam.com/utils/rpc"

	"go.viam.com/rdk/component/base"
	"go.viam.com/rdk/grpc/client"
	"go.viam.com/rdk/motionplan"
	frame "go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/resource"
	spatial "go.viam.com/rdk/spatialmath"
	utilsrdk "go.viam.com/rdk/utils"
)

var logger = golog.NewDevelopmentLogger("agile")

func main() {
	utils.ContextualMain(mainWithArgs, logger)
}

type obstacle struct {
	Center []float64 `json:"center"`
	Dims   []float64 `json:"dims"`
}

// MobileRobotPlanConfig describes a motion planning problem for a 2D mobile robot.
type MobileRobotPlanConfig struct {
	Name           string  `json:"name"`
	GridConversion float64 `json:"grid-conversion"` // in mm
	Type           string  `json:"type"`
	// planning conditions
	Start []float64 `json:"start"`
	Goal  []float64 `json:"goal"`

	// robot params
	RobotDims []float64 `json:"robot-dims"`
	Radius    float64   `json:"radius"`
	PointSep  float64   `json:"point-sep"`

	// map definition
	Xlim      []float64  `json:"xlim"`
	YLim      []float64  `json:"ylim"`
	Obstacles []obstacle `json:"obstacles"`
}

func mainWithArgs(ctx context.Context, args []string, logger golog.Logger) error {
	move := false

	if move {
		robot, err := client.New(
			context.Background(),
			args[1],
			logger,
			client.WithDialOptions(rpc.WithCredentials(rpc.Credentials{
				Type:    utilsrdk.CredentialsTypeRobotLocationSecret,
				Payload: args[2],
			})),
		)
		if err != nil {
			logger.Debug(err)
			return err
		}
		defer robot.Close(ctx)
		logger.Info("Resources:")
		logger.Info(robot.ResourceNames())

		l, err := robot.ResourceByName(resource.NameFromSubtype(base.Subtype, "limo"))
		if err != nil {
			return err
		}
		b := l.(base.Base)
		limo1 := limoBase{ctx: ctx, realBase: b, driveMode: "ackermann", logger: logger}

		// read config
		config, err := parseJSONFile("samples/agile/planConfig.json")
		if err != nil {
			logger.Fatal(err.Error())
		}

		// call Move from Agile
		limo1.Move(config)
	} else {
		limo1 := limoBase{ctx: ctx, driveMode: "ackermann", logger: logger}

		// read config
		config, err := parseJSONFile("samples/agile/planConfig.json")
		if err != nil {
			logger.Fatal(err.Error())
		}

		// call Plan from Agile
		waypoints, d, err := limo1.Plan(config)
		if err != nil {
			return err
		}

		// write output
		if err := writeJSONFile("samples/agile/planOutput.json", waypoints); err != nil {
			logger.Fatal(err.Error())
		}

		savePath(d, waypoints, config)
	}

	return nil
}

type limoBase struct {
	ctx       context.Context
	realBase  base.Base
	driveMode string
	logger    golog.Logger
}

func (l *limoBase) Spin(angleDeg, degsPerSec float64) {
	l.realBase.Spin(l.ctx, angleDeg, degsPerSec, nil)
}

func (l *limoBase) MoveStraight(distanceMm int, mmPerSec float64) {
	l.realBase.MoveStraight(l.ctx, distanceMm, mmPerSec, nil)
}

func (l *limoBase) Move(planConfig *MobileRobotPlanConfig) error {
	switch l.driveMode {
	case "ackermann":
		waypoints, d, err := l.Plan(planConfig)
		if err != nil {
			return err
		}
		traj := motionplan.GetDubinTrajectoryFromPath(waypoints, d)
		l.FollowDubinsTrajectory(planConfig, traj)
	default:
		l.logger.Info("no motion plan type specified")
	}

	return nil
}

func (l *limoBase) newDubinsPlanner(config *MobileRobotPlanConfig) (*motionplan.DubinsRRTMotionPlanner, error) {
	// parse input
	robotGeometry, err := spatial.NewBoxCreator(r3.Vector{X: config.RobotDims[0], Y: config.RobotDims[1], Z: 1}, spatial.NewZeroPose())
	if err != nil {
		return nil, err
	}
	limits := []frame.Limit{{Min: config.Xlim[0], Max: config.Xlim[1]}, {Min: config.YLim[0], Max: config.YLim[1]}}
	// TODO(rb) add logic to parse limit input to check for infinite limits
	// limits = []frame.Limit{{Min: math.Inf(-1), Max: math.Inf(1)}, {Min: math.Inf(-1), Max: math.Inf(1)}}

	// build model
	model, err := frame.NewMobile2DFrame(config.Name, limits, robotGeometry)
	if err != nil {
		return nil, err
	}

	// setup planner
	radius := config.Radius / config.GridConversion
	d := motionplan.Dubins{Radius: radius, PointSeparation: config.PointSep}
	mp, err := motionplan.NewDubinsRRTMotionPlanner(model, 1, l.logger, d)
	if err != nil {
		return nil, err
	}

	dubins, ok := mp.(*motionplan.DubinsRRTMotionPlanner)
	if !ok {
		return nil, errors.New("Could not create DubinsRRTMotionPlanner")
	}
	return dubins, nil
}

func (l *limoBase) Plan(config *MobileRobotPlanConfig) ([][]frame.Input, motionplan.Dubins, error) {
	dubins, err := l.newDubinsPlanner(config)
	if err != nil {
		return nil, motionplan.Dubins{}, err
	}

	start := frame.FloatsToInputs(config.Start)
	goal := spatial.PoseToProtobuf(spatial.NewPoseFromPoint(r3.Vector{X: config.Goal[0], Y: config.Goal[1], Z: 0}))
	obstacleGeometries := map[string]spatial.Geometry{}
	for i, o := range config.Obstacles {
		box, err := spatial.NewBox(spatial.NewPoseFromPoint(
			r3.Vector{X: o.Center[0], Y: o.Center[1], Z: 0}),
			r3.Vector{X: o.Dims[0], Y: o.Dims[1], Z: 1})
		if err != nil {
			return nil, motionplan.Dubins{}, err
		}
		obstacleGeometries[strconv.Itoa(i)] = box
	}

	opt := motionplan.NewBasicPlannerOptions()
	opt.AddConstraint("collision", motionplan.NewCollisionConstraint(dubins.Frame(), obstacleGeometries, map[string]spatial.Geometry{}))

	// plan
	waypoints, err := dubins.Plan(l.ctx, goal, start, opt)
	if err != nil {
		return nil, motionplan.Dubins{}, err
	}
	return waypoints, dubins.D, nil
}

func (l *limoBase) FollowDubinsTrajectory(config *MobileRobotPlanConfig, traj []motionplan.DubinPathAttr) {
	for _, opt := range traj {
		dubinsPath := opt.DubinsPath
		straight := opt.Straight

		l.MoveToWaypointDubins(config, dubinsPath, straight)
	}
}

func fixAngle(ang float64) float64 {
	// currently positive angles move base clockwise, which is opposite of what is expected, so multiply by -1
	deg := -ang * 180 / math.Pi
	return deg
}

func (l *limoBase) MoveToWaypointDubins(config *MobileRobotPlanConfig, path []float64, straight bool) {
	// first turn
	l.Spin(fixAngle(path[0]), 20) // base is currently configured backwards

	// second turn/straight
	if straight {
		l.MoveStraight(int(path[2]*config.GridConversion), 100) // constant speed right now
	} else {
		l.Spin(fixAngle(path[2]), 20)
	}

	// last turn
	l.Spin(fixAngle(path[1]), 40)
}

func savePath(d motionplan.Dubins, waypoints [][]frame.Input, config *MobileRobotPlanConfig) error {
	csvFile, err := os.Create("path.csv")
	if err != nil {
		return err
	}
	csvwriter := csv.NewWriter(csvFile)

	start := make([]float64, 3)
	next := make([]float64, 3)

	for i, wp := range waypoints {
		if i == 0 {
			for j := 0; j < 3; j++ {
				start[j] = wp[j].Value
			}
		} else {
			for j := 0; j < 3; j++ {
				next[j] = wp[j].Value
			}

			pathOptions := d.AllPaths(start, next, true)[0]

			dubinsPath := pathOptions.DubinsPath

			sstra := "0"
			last := fixAngle(dubinsPath[2])
			if pathOptions.Straight {
				sstra = "1"
				last = dubinsPath[2]
			}

			writeData := []string{
				fmt.Sprintf("%f",
					fixAngle(start[0])),
				fmt.Sprintf("%f",
					fixAngle(start[1])),
				fmt.Sprintf("%f",
					fixAngle(start[2])),
				fmt.Sprintf("%f",
					fixAngle(dubinsPath[0])),
				fmt.Sprintf("%f", fixAngle(dubinsPath[1])),
				fmt.Sprintf("%f", last),
				sstra,
			}
			_ = csvwriter.Write(writeData)

			for j := 0; j < 3; j++ {
				start[j] = next[j]
			}
		}
	}
	// last point
	writeData := []string{
		fmt.Sprintf("%f", start[0]),
		fmt.Sprintf("%f", start[1]),
		fmt.Sprintf("%f", start[2]),
		fmt.Sprintf("%d", 0),
		fmt.Sprintf("%d", 0),
		fmt.Sprintf("%d", 0),
		fmt.Sprintf("%d", 0),
	}
	_ = csvwriter.Write(writeData)

	csvwriter.Flush()
	csvFile.Close()

	// now write obstacles if there are obstacles
	csvFile, err = os.Create("obstacles.csv")
	if err != nil {
		return err
	}
	csvwriter = csv.NewWriter(csvFile)

	for _, o := range config.Obstacles {
		writeData := []string{
			fmt.Sprintf("%f", o.Center[0]),
			fmt.Sprintf("%f", o.Center[1]),
			fmt.Sprintf("%f", o.Dims[0]),
			fmt.Sprintf("%f", o.Dims[1]),
		}
		_ = csvwriter.Write(writeData)
	}

	csvwriter.Flush()
	csvFile.Close()

	return nil
}

func parseJSONFile(filename string) (*MobileRobotPlanConfig, error) {
	jsonData, err := os.ReadFile(filename)
	if err != nil {
		return nil, errors.Wrap(err, "failed to read json file")
	}
	config := &MobileRobotPlanConfig{}
	if len(jsonData) == 0 {
		return nil, errors.New("no model information")
	}
	err = json.Unmarshal(jsonData, config)
	if err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal json file")
	}

	// assert correctness of json
	wrongDimsError := errors.New("need array of floats to have exactly 2 elements")
	wrongDimsError3 := errors.New("need array of floats to have exactly 3 elements")
	if len(config.Start) != 3 {
		return nil, errors.Wrap(wrongDimsError3, "config error in start field")
	}
	if len(config.Goal) != 3 {
		return nil, errors.Wrap(wrongDimsError3, "config error in start field")
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
	if config.GridConversion == 0 {
		config.GridConversion = 1
	}
	return config, nil
}

func writeJSONFile(filename string, data interface{}) error {
	bytes, err := json.MarshalIndent(data, "", " ")
	if err != nil {
		return err
	}
	_, err = os.Create(filename)
	if err != nil {
		return err
	}
	if err := os.WriteFile(filename, bytes, 0o644); err != nil {
		return err
	}
	return nil
}
