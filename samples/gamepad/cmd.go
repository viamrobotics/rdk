package main

import (
	"context"
	"flag"
	"time"

	"github.com/edaniels/golog"
	"go.viam.com/utils"
	"go.viam.com/utils/rpc"

	"go.viam.com/rdk/component/input"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/grpc/client"
	"go.viam.com/rdk/metadata/service"
	"go.viam.com/rdk/robot"
	robotimpl "go.viam.com/rdk/robot/impl"
	"go.viam.com/rdk/services/web"
	webserver "go.viam.com/rdk/web/server"
)

var logger = golog.NewDevelopmentLogger("gamepad")

func main() {
	utils.ContextualMain(mainWithArgs, logger)
}

func mainWithArgs(ctx context.Context, args []string, logger golog.Logger) (err error) {
	flag.Parse()

	cfg, err := config.Read(ctx, flag.Arg(0))
	if err != nil {
		return err
	}

	metadataSvc, err := service.New()
	if err != nil {
		return err
	}
	ctx = service.ContextWithService(ctx, metadataSvc)

	myRobot, err := robotimpl.New(ctx, cfg, logger, client.WithDialOptions(rpc.WithInsecure()))
	if err != nil {
		return err
	}
	defer myRobot.Close(ctx)
	go debugOut(ctx, myRobot)

	return webserver.RunWeb(ctx, myRobot, web.NewOptions(), logger)
}

func debugOut(ctx context.Context, r robot.Robot) {
	g, ok := r.InputControllerByName("Mux")
	if !ok {
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
			logger.Infof("%s:%s: %.4f\n", event.Control, event.Event, event.Value)
			err = g.RegisterControlCallback(ctx, control, []input.EventType{input.AllEvents}, repFunc)
			if err != nil {
				return
			}
		}
		break
	}
}
