package main

import (
	"context"
	"flag"
	"fmt"
	"time"

	"github.com/go-errors/errors"
	"go.viam.com/utils"

	"go.viam.com/core/config"
	"go.viam.com/core/input"
	"go.viam.com/core/robot"

	robotimpl "go.viam.com/core/robot/impl"
	"go.viam.com/core/web"
	webserver "go.viam.com/core/web/server"

	"github.com/edaniels/golog"
)

var logger = golog.NewDevelopmentLogger("gamepad")

func main() {
	utils.ContextualMain(mainWithArgs, logger)
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
	go debugOut(ctx, myRobot)
	webOpts := web.NewOptions()
	webOpts.Insecure = true

	err = webserver.RunWeb(ctx, myRobot, webOpts, logger)
	if err != nil && !errors.Is(err, context.Canceled) {
		logger.Errorw("error running web", "error", err)
		return err
	}

	return nil
}

func debugOut(ctx context.Context, r robot.Robot) {
	g, ok := r.InputControllerByName("TestGamepad")
	if !ok {
		return
	}

	repFunc := func(ctx context.Context, event input.Event) {
		fmt.Printf("%s:%s: %.4f\n", event.Control, event.Event, event.Value)
	}

	// Expects auto_reconnect to be set in the config
	for {
		if !utils.SelectContextOrWait(ctx, time.Second) {
			return
		}
		controls, err := g.Controls(ctx)
		if err != nil {
			logger.Error(err)
			continue
		}

		lastEvents, err := g.LastEvents(ctx)
		if err != nil {
			return
		}
		for _, control := range controls {
			event := lastEvents[control]
			fmt.Printf("%s:%s: %.4f\n", event.Control, event.Event, event.Value)
			err = g.RegisterControlCallback(ctx, control, []input.EventType{input.AllEvents}, repFunc)
			if err != nil {
				return
			}
		}
		break
	}
}
