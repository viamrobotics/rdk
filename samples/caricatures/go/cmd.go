package main

import (
	"context"
	"errors"
	"flag"

	"github.com/edaniels/golog"

	"go.viam.com/core/config"
	"go.viam.com/core/robot"
	robotimpl "go.viam.com/core/robot/impl"
	"go.viam.com/core/web"
	webserver "go.viam.com/core/web/server"
)

const (
	NUM_FACIAL_LANDMARKS = 68
)

var logger = golog.NewDevelopmentLogger("caricatures")

func drawPoint(ctx context.Context, r robot.Robot) error {
	if len(r.ArmNames()) != 1 {
		return errors.New("need 1 arm name")
	}
	arm := r.ArmByName(r.ArmNames()[0])

	for i := 0; i < NUM_FACIAL_LANDMARKS; i++ {
		pos, err := arm.CurrentPosition(ctx)
		if err != nil {
			return err
		}
		arm.MoveToPosition(ctx, pos)
	}
	return nil
}

func main() {
	// utils.ContextualMain(mainWithArgs, logger)
	// setupFaceFromJSON()
	createPlotsAndRegressions()
	print("\n\nSTARTING CARICATURES\n\n")

}

func mainWithArgs(ctx context.Context, args []string, logger golog.Logger) (err error) {

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

// return (errors.New("\n\nMAX\n\n"))
