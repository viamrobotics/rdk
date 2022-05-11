package main

import (
	"context"
	"flag"
	"time"

	"github.com/edaniels/golog"
	"go.viam.com/utils"

	"go.viam.com/rdk/component/input"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/robot"
	robotimpl "go.viam.com/rdk/robot/impl"
	"go.viam.com/rdk/robot/web"
)

var logger = golog.NewDevelopmentLogger("inputtest")

func main() {
	utils.ContextualMain(mainWithArgs, logger)
}

func mainWithArgs(ctx context.Context, args []string, logger golog.Logger) (err error) {
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
	go debugOut(ctx, myRobot)

	return web.RunWebWithConfig(ctx, myRobot, cfg, logger)
}

func debugOut(ctx context.Context, r robot.Robot) {
	g, err := input.FromRobot(r, "Mux")
	if err != nil {
		return
	}

	repFunc := func(ctx context.Context, event input.Event) {
		logger.Infof("%s:%s: %.4f\n", event.Control, event.Event, event.Value)
	}

	// Expects auto_reconnect to be set in the config
	for {
		if !utils.SelectContextOrWait(ctx, time.Second) {
			return
		}
		controls, err := g.GetControls(ctx)
		if err != nil {
			logger.Error(err)
			continue
		}

		lastEvents, err := g.GetEvents(ctx)
		if err != nil {
			return
		}
		for _, control := range controls {
			event := lastEvents[control]
			logger.Infof("%s:%s: %.4f\n", event.Control, event.Event, event.Value)
			err = g.RegisterControlCallback(ctx, control, []input.EventType{input.AllEvents}, repFunc)
			if err != nil {
				return
			}
		}
		break
	}
}
