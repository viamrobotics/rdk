package main

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math"
	"os"

	"github.com/edaniels/golog"
	"github.com/pkg/errors"
	"go.viam.com/rdk/grpc/client"

	"go.viam.com/utils"

	"go.viam.com/rdk/component/base"
	limo "go.viam.com/rdk/component/base/agilex"
	"go.viam.com/rdk/resource"
	utilsrdk "go.viam.com/rdk/utils"
	"go.viam.com/utils/rpc"

	"go.viam.com/rdk/motionplan"
	frame "go.viam.com/rdk/referenceframe"
)

var logger = golog.NewDevelopmentLogger("agile")

const (
	gridConversion = 1000 // mm per grid square
)

func main() {
	utils.ContextualMain(mainWithArgs, logger)
}

func mainWithArgs(ctx context.Context, args []string, logger golog.Logger) error {
	move := true
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
	b := l.(base.Base)
	limo1, err := limo.GetLimoBase(b)
	if err != nil {
		return err
	}

	if move {
		// read config
		config, err := parseJSONFile("samples/agile/planConfig.json")
		if err != nil {
			logger.Fatal(err.Error())
		}

		// call Move from Agile
		limo1.Move(ctx, config, logger)
	} else {
		// read config
		config, err := parseJSONFile("samples/agile/planConfig.json")
		if err != nil {
			logger.Fatal(err.Error())
		}

		// call Plan from Agile
		waypoints, d, err := limo1.Plan(ctx, config, logger)
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

func savePath(d motionplan.Dubins, waypoints [][]frame.Input, config *motionplan.MobileRobotPlanConfig) error {
	withAgile := false

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

			pathOptions := d.AllOptions(start, next, true)[0]

			dubinsPath := pathOptions.DubinsPath

			sstra := "0"
			last := fixAngle(dubinsPath[2], withAgile)
			if pathOptions.Straight {
				sstra = "1"
				last = dubinsPath[2]
			}

			writeData := []string{fmt.Sprintf("%f", fixAngle(start[0], withAgile)), fmt.Sprintf("%f", fixAngle(start[1], withAgile)), fmt.Sprintf("%f", fixAngle(start[2], withAgile)), fmt.Sprintf("%f", fixAngle(dubinsPath[0], withAgile)), fmt.Sprintf("%f", fixAngle(dubinsPath[1], withAgile)), fmt.Sprintf("%f", last), sstra}
			_ = csvwriter.Write(writeData)

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

	//now write obstacles if there are obstacles
	csvFile, err = os.Create("obstacles.csv")
	if err != nil {
		return err
	}
	csvwriter = csv.NewWriter(csvFile)

	for _, o := range config.Obstacles {
		writeData := []string{fmt.Sprintf("%f", o.Center[0]), fmt.Sprintf("%f", o.Center[1]), fmt.Sprintf("%f", o.Dims[0]), fmt.Sprintf("%f", o.Dims[1])}
		_ = csvwriter.Write(writeData)
	}

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

func parseJSONFile(filename string) (*motionplan.MobileRobotPlanConfig, error) {
	jsonData, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, errors.Wrap(err, "failed to read json file")
	}
	config := &motionplan.MobileRobotPlanConfig{}
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
