package main

import (
	"context"
	"errors"
	"flag"
	"fmt"

	"github.com/edaniels/golog"
	"go.viam.com/utils"
	"go.viam.com/utils/rpc"

	"go.viam.com/core/action"
	"go.viam.com/core/config"
	"go.viam.com/core/grpc/client"
	"go.viam.com/core/robot"
	robotimpl "go.viam.com/core/robot/impl"
	"go.viam.com/core/services/web"
	webserver "go.viam.com/core/web/server"
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

// drawPoint instructs a robot to draw by moving its arm into specific positions sequentially
func drawPoint(ctx context.Context, r robot.Robot) error {
	if len(r.ArmNames()) != 1 {
		return errors.New("need 1 arm name")
	}
	arm, ok := r.ArmByName(r.ArmNames()[0])
	if !ok {
		return fmt.Errorf("failed to find arm %q", r.ArmNames()[0])
	}

	for i := 0; i < numFacialLandmarks; i++ {
		pos, err := arm.CurrentPosition(ctx)
		if err != nil {
			return err
		}
		arm.MoveToPosition(ctx, pos)
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
		cfg, err := config.Read(flag.Arg(0))
		if err != nil {
			return err
		}
		myRobot, err := robotimpl.New(ctx, cfg, logger, client.WithDialOptions(rpc.WithInsecure()))
		if err != nil {
			return err
		}
		defer myRobot.Close()
		return webserver.RunWeb(ctx, myRobot, web.NewOptions(), logger)
	}
	return nil
}
