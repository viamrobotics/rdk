package main

import (
  "context"
  "math"
  "fmt"
  "encoding/json"
	"io/ioutil"
	"strconv"

  "github.com/edaniels/golog"
  "go.viam.com/rdk/grpc/client"
  "go.viam.com/utils"
  "github.com/golang/geo/r3"
	"github.com/pkg/errors"

  utilsrdk "go.viam.com/rdk/utils"
  "go.viam.com/utils/rpc"
  "go.viam.com/rdk/component/base"
  "go.viam.com/rdk/resource"
	"go.viam.com/rdk/motionplan"
	frame "go.viam.com/rdk/referenceframe"
	spatial "go.viam.com/rdk/spatialmath"

)

var logger = golog.NewDevelopmentLogger("agile")



func main() {
	utils.ContextualMain(mainWithArgs, logger)
}

func mainWithArgs(ctx context.Context, args []string, logger golog.Logger) error {
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
	waypoints, err := plan(ctx, config)
	if err != nil {
		logger.Fatal(err.Error())
	}

	// write output
	if err := writeJSONFile("samples/agile/planOutput.json", waypoints); err != nil {
		logger.Fatal(err.Error())
	}

  limo, err := robot.ResourceByName(resource.NameFromSubtype(base.Subtype, "limo"))

  limo1 := limo.(base.Base)

  x1, y1, x2, y2 := 0., 0., 0., 0.

  for i, wp := range waypoints {
    if i == 0 {
      x1 = wp[0].Value
      y1 = wp[1].Value
    } else {
      x2 = wp[0].Value
      y2 = wp[1].Value
      MoveToWaypoint(ctx, limo1, x1, y1, x2, y2)

      x1, y1 = x2, y2
    }
  }

  return nil
  
}

func MoveToWaypoint(ctx context.Context, limo base.Base, x1 float64, y1 float64, x2 float64, y2 float64) error {
  dir := 0.0  //get direction from state, but assume we're pointing at x direction for rn
  dist := math.Sqrt(math.Pow((y2-y1), 2) + math.Pow((x2-x1), 2))*500  //each grid square is half a meter
  theta := math.Acos((x2-x1)/dist)  //angle to x axis

  moves := base.Move{DistanceMm: int(dist), MmPerSec: 100, AngleDeg: (dir-theta)*(180/math.Pi), DegsPerSec: 20}
  err := base.DoMove(ctx, moves, limo)
  if err!=nil {
    return err
  }

  return nil

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

	// map definition
	Xlim      []float64  `json:"xlim"`
	YLim      []float64  `json:"ylim"`
	Obstacles []obstacle `json:"obstacles"`
}

func plan(ctx context.Context, config *mobileRobotPlanConfig) ([][]frame.Input, error) {
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
	cbert, err := motionplan.NewCBiRRTMotionPlanner(model, 1, logger)
	if err != nil {
		return nil, err
	}
	opt := motionplan.NewDefaultPlannerOptions()
	opt.AddConstraint("collision", motionplan.NewCollisionConstraint(model, obstacleGeometries, map[string]spatial.Geometry{}))

	// plan
	waypoints, err := cbert.Plan(ctx, goal, start, opt)
	if err != nil {
		return nil, err
	}
	return waypoints, nil
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

