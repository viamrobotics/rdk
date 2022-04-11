package main

import (
	"context"
	"errors"
	"flag"

	"github.com/edaniels/golog"
	"go.viam.com/utils"

	"go.viam.com/rdk/action"
	"go.viam.com/rdk/component/arm"
	"go.viam.com/rdk/config"
	commonpb "go.viam.com/rdk/proto/api/common/v1"
	"go.viam.com/rdk/robot"
	robotimpl "go.viam.com/rdk/robot/impl"
	"go.viam.com/rdk/services/web"
)

const (
	numFacialLandmarks = 68
	personToDraw       = "person"
)

var logger = golog.NewDevelopmentLogger("caricatures")

func init() {
	// TODO(mh): actually register action
	if false {
		action.RegisterAction("drawPoint", func(ctx context.Context, r robot.Robot) {
			if err := drawPoint(ctx, r); err != nil {
				logger.Errorw("Error running drawPoint:", "error", err)
			}
		})
	}
}

// drawPoint instructs a robot to draw by moving its arm into specific positions sequentially.
func drawPoint(ctx context.Context, r robot.Robot) error {
	if len(arm.NamesFromRobot(r)) != 1 {
		return errors.New("need 1 arm name")
	}
	a, err := arm.FromRobot(r, arm.NamesFromRobot(r)[0])
	if err != nil {
		return err
	}

	for i := 0; i < numFacialLandmarks; i++ {
		pos, err := a.GetEndPosition(ctx)
		if err != nil {
			return err
		}
		a.MoveToPosition(ctx, pos, &commonpb.WorldState{})
	}
	return nil
}

func main() {
	utils.ContextualMain(mainWithArgs, logger)
}

func mainWithArgs(ctx context.Context, args []string, logger golog.Logger) error {
	// use built-in camera to find a face and create its caricature
	if err := findFace(personToDraw); err != nil {
		return err
	}
	if err := createPlots(personToDraw); err != nil {
		return err
	}

	if false {
		flag.Parse()
		cfg, err := config.Read(ctx, flag.Arg(0), logger)
		if err != nil {
			return err
		}
		myRobot, err := robotimpl.RobotFromConfig(ctx, cfg, logger)
		if err != nil {
			return err
		}
		defer myRobot.Close(ctx)
		return web.RunWebWithConfig(ctx, myRobot, cfg, logger)
	}
	return nil
}
