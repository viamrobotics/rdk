package main

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math"
	"os"
	"strconv"

	"github.com/edaniels/golog"
	"github.com/golang/geo/r3"
	"github.com/pkg/errors"
	"go.viam.com/rdk/grpc/client"

	"go.viam.com/rdk/resource"
	"go.viam.com/utils"

	"go.viam.com/rdk/component/base"
	utilsrdk "go.viam.com/rdk/utils"
	"go.viam.com/utils/rpc"

	"go.viam.com/rdk/motionplan"
	frame "go.viam.com/rdk/referenceframe"
	spatial "go.viam.com/rdk/spatialmath"
)

var logger = golog.NewDevelopmentLogger("agile")

const (
	gridConversion = 500 // mm per grid square
)

func main() {
	utils.ContextualMain(mainWithArgs, logger)
}

func mainWithArgs(ctx context.Context, args []string, logger golog.Logger) error {
	withAgile := false
	collectData := true

	if collectData {
		robot, err := client.New(
			context.Background(),
			"agilex-limo-main.60758fe0f6.viam.cloud",
			logger,
			client.WithDialOptions(rpc.WithCredentials(rpc.Credentials{
				Type:    utilsrdk.CredentialsTypeRobotLocationSecret,
				Payload: "pem1epjv07fq2cz2z5723gq6ntuyhue5t30boohkiz3iqht4",
			})),
		)
		if err != nil {
			logger.Fatal(err)
		}
		defer robot.Close(context.Background())
		logger.Info("Resources:")
		logger.Info(robot.ResourceNames())

		// limo, err := robot.ResourceByName(resource.NameFromSubtype(base.Subtype, "limo"))
		// limo1 := limo.(base.Base)

		// limo1.Spin(ctx, 360, 20)
		// time.Sleep(time.Millisecond * 2000)

		// limo1.Spin(ctx, -360, 20)
		// time.Sleep(time.Millisecond * 2000)

		// limo1.Spin(ctx, 360, 20)
		// time.Sleep(time.Millisecond * 2000)

		// limo1.Spin(ctx, -360, 20)
		// time.Sleep(time.Millisecond * 2000)

		return nil
	}

	if withAgile {
		robot, err := client.New(
			ctx,
			"agilex-limo-main.60758fe0f6.viam.cloud",
			logger,
			client.WithDialOptions(rpc.WithCredentials(rpc.Credentials{
				Type:    utilsrdk.CredentialsTypeRobotLocationSecret,
				Payload: "pem1epjv07fq2cz2z5723gq6ntuyhue5t30boohkiz3iqht4",
			})),
		)
		if err != nil {
			logger.Debug(err)
			return err
		}
		defer robot.Close(ctx)
		logger.Info("Resources:")
		logger.Info(robot.ResourceNames())

		// read config
		config, err := parseJSONFile("samples/agile/planConfig.json")
		if err != nil {
			logger.Fatal(err.Error())
		}

		// plan
		d, waypoints, err := plan(ctx, config)
		if err != nil {
			logger.Fatal(err.Error())
		}

		// write output
		if err := writeJSONFile("samples/agile/planOutput.json", waypoints); err != nil {
			logger.Fatal(err.Error())
		}

		limo, err := robot.ResourceByName(resource.NameFromSubtype(base.Subtype, "limo"))

		limo1 := limo.(base.Base)

		start := make([]float64, 3)
		next := make([]float64, 3)

		savePath(d, waypoints)

		for i, wp := range waypoints {
			if i == 0 {
				for j := 0; j < 3; j++ {
					start[j] = wp[j].Value
				}
			} else {
				for j := 0; j < 3; j++ {
					next[j] = wp[j].Value
				}

				pathOptions := d.AllOptions(start, next, true)[0]

				dubinsPath := pathOptions.DubinsPath
				straight := pathOptions.Straight

				fmt.Println("start: ", start)
				fmt.Println("next: ", next)

				MoveToWaypointDubins(ctx, limo1, dubinsPath, straight)

				for j := 0; j < 3; j++ {
					start[j] = next[j]
				}
			}
		}

	} else {
		// read config
		config, err := parseJSONFile("samples/agile/planConfig.json")
		if err != nil {
			logger.Fatal(err.Error())
		}

		// plan
		d, waypoints, err := plan(ctx, config)
		if err != nil {
			logger.Fatal(err.Error())
		}

		// write output
		if err := writeJSONFile("samples/agile/planOutput.json", waypoints); err != nil {
			logger.Fatal(err.Error())
		}

		savePath(d, waypoints)
	}

	return nil

}

func savePath(d motionplan.Dubins, waypoints [][]frame.Input) error {
	withAgile := false

	csvFile, err := os.Create("/home/skarpoor12/data/motion/path.csv")
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

			pathOptions := d.AllOptions(start, next, true)[0]

			dubinsPath := pathOptions.DubinsPath
			fmt.Println("FINALPATH: ", dubinsPath)

			sstra := "0"
			last := fixAngle(dubinsPath[2], withAgile)
			if pathOptions.Straight {
				sstra = "1"
				last = dubinsPath[2]
			}

			writeData := []string{fmt.Sprintf("%f", fixAngle(start[0], withAgile)), fmt.Sprintf("%f", fixAngle(start[1], withAgile)), fmt.Sprintf("%f", fixAngle(start[2], withAgile)), fmt.Sprintf("%f", fixAngle(dubinsPath[0], withAgile)), fmt.Sprintf("%f", fixAngle(dubinsPath[1], withAgile)), fmt.Sprintf("%f", last), sstra}
			_ = csvwriter.Write(writeData)
			fmt.Println("WRITING: ", writeData)

			for j := 0; j < 3; j++ {
				start[j] = next[j]
			}
		}
	}
	//last point
	writeData := []string{fmt.Sprintf("%f", start[0]), fmt.Sprintf("%f", start[1]), fmt.Sprintf("%f", start[2]), fmt.Sprintf("%d", 0), fmt.Sprintf("%d", 0), fmt.Sprintf("%d", 0), fmt.Sprintf("%d", 0)}
	_ = csvwriter.Write(writeData)

	csvwriter.Flush()
	csvFile.Close()
	return nil
}

func fixAngle(ang float64, withAgile bool) float64 {
	// angle should be between principle of values: -90 to 90 degrees
	if !withAgile {
		return ang
	}
	deg := ang * 180 / math.Pi
	return deg

}

func MoveToWaypointDubins(ctx context.Context, limo base.Base, path []float64, straight bool) {
	//first turn
	limo.Spin(ctx, -fixAngle(path[0], true), 20) //base is currently configured backwards

	//second turn/straight
	if straight {
		limo.MoveStraight(ctx, int(path[2]*gridConversion), 100)
	} else {
		limo.Spin(ctx, -fixAngle(path[2], true), 20)
	}

	//last turn
	limo.Spin(ctx, -fixAngle(path[1], true), 40)
}

func MoveToWaypoint(ctx context.Context, limo base.Base, x1 float64, y1 float64, x2 float64, y2 float64, dir float64) (float64, error) {
	dist := math.Sqrt(math.Pow((y2-y1), 2) + math.Pow((x2-x1), 2)) //in grid values
	theta := math.Acos((x2-x1)/dist) * (180 / math.Pi)             //angle to x axis in degrees

	//   turnRadius := int(322/math.Tan(.48869)) //in mm, turning angle is about 28 degrees right now

	newAngle := dir - theta
	moves := base.Move{DistanceMm: int(dist) * 1000, MmPerSec: 100, AngleDeg: newAngle, DegsPerSec: 20}
	err := base.DoMove(ctx, moves, limo)
	if err != nil {
		return dir, err
	}

	return math.Mod((dir + newAngle), 360), nil

}

type obstacle struct {
	Center []float64 `json:"center"`
	Dims   []float64 `json:"dims"`
}

type mobileRobotPlanConfig struct {
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

func plan(ctx context.Context, config *mobileRobotPlanConfig) (motionplan.Dubins, [][]frame.Input, error) {
	// parse input
	start := frame.FloatsToInputs(config.Start)
	goal := spatial.PoseToProtobuf(spatial.NewPoseFromPoint(r3.Vector{X: config.Goal[0], Y: config.Goal[1], Z: 0}))
	robotGeometry, err := spatial.NewBoxCreator(r3.Vector{X: config.RobotDims[0], Y: config.RobotDims[1], Z: 1}, spatial.NewZeroPose())
	if err != nil {
		return motionplan.Dubins{}, nil, err
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
			return motionplan.Dubins{}, nil, err
		}
		obstacleGeometries[strconv.Itoa(i)] = box
	}

	// build model
	model, err := frame.NewMobile2DFrame("mobile-base", limits, robotGeometry)
	if err != nil {
		return motionplan.Dubins{}, nil, err
	}

	// setup planner
	radius := config.Radius * 1000.0 / gridConversion
	fmt.Println("Radius", radius)
	d := motionplan.Dubins{Radius: radius, PointSeparation: config.PointSep}
	dubins, err := motionplan.NewDubinsRRTMotionPlanner(model, 1, logger, d)
	if err != nil {
		return motionplan.Dubins{}, nil, err
	}
	opt := motionplan.NewDefaultPlannerOptions()
	opt.AddConstraint("collision", motionplan.NewCollisionConstraint(model, obstacleGeometries, map[string]spatial.Geometry{}))

	// plan
	waypoints, err := dubins.Plan(ctx, goal, start, opt)
	if err != nil {
		return motionplan.Dubins{}, nil, err
	}

	return d, waypoints, nil
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
