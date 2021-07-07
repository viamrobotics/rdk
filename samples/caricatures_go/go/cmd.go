package main

import (
	"context"
	"errors"
	"flag"

	"github.com/edaniels/golog"
	"go.viam.com/utils"

	"go.viam.com/core/action"
	"go.viam.com/core/config"
	"go.viam.com/core/robot"
	robotimpl "go.viam.com/core/robot/impl"
	"go.viam.com/core/web"
	webserver "go.viam.com/core/web/server"
)

// constants
const (
	numFacialLandmarks = 68
)

// initialize logger for caricatures
var logger = golog.NewDevelopmentLogger("caricatures")

// init initializes the robot and provides test actions to complete
func init() {
	// IF FALSE MAKES SURE BELOW CODE PASSES LINTER BECAUSE IT IS UNUSED
	if false {
		action.RegisterAction("drawPoint", func(ctx context.Context,
			r robot.Robot) {
			err := drawPoint(ctx, r)
			if err != nil {
				logger.Errorf("Error running drawPoint: %s", err)
			}
		})
	}
}

// drawPoint instructs a robot to draw by moving its arm
// into specific positions sequentially
func drawPoint(ctx context.Context, r robot.Robot) error {
	if len(r.ArmNames()) != 1 {
		return errors.New("need 1 arm name")
	}
	arm := r.ArmByName(r.ArmNames()[0])

	for i := 0; i < numFacialLandmarks; i++ {
		pos, err := arm.CurrentPosition(ctx)
		if err != nil {
			return err
		}
		arm.MoveToPosition(ctx, pos)
	}
	return nil
}

// main method
func main() {

	// IF FALSE MAKES SURE BELOW CODE PASSES LINTER BECAUSE IT IS UNUSED
	if false {
		utils.ContextualMain(mainWithArgs, logger)
	}
	// create caricature curves and plot those curves to create a visual
	// representation of a face
	createPlots()

}

// mainWithArgs method used to initialize the robot
func mainWithArgs(ctx context.Context, args []string, logger golog.Logger) error {
	flag.Parse()
	cfg, err := config.Read(flag.Arg(0))
	if err != nil {
		return err
	}
	myRobot, err := robotimpl.New(ctx, cfg, logger)
	if err != nil {
		return err
	}
	defer myRobot.Close()
	webOpts := web.NewOptions()
	webOpts.Insecure = true
	return webserver.RunWeb(ctx, myRobot, webOpts, logger)
}
