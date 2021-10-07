package main

import (
	"context"
	"flag"
	"fmt"
	"time"

	"github.com/go-errors/errors"
	"go.viam.com/utils"

	"go.viam.com/core/config"
	//"go.viam.com/core/input"
	"go.viam.com/core/robot"

	robotimpl "go.viam.com/core/robot/impl"
	"go.viam.com/core/web"
	webserver "go.viam.com/core/web/server"

	"github.com/edaniels/golog"
)

var logger = golog.NewDevelopmentLogger("gamepadtest")

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

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	go debugOut(ctx, myRobot)
	webOpts := web.NewOptions()
	webOpts.Insecure = true

	err = webserver.RunWeb(ctx, myRobot, webOpts, logger)
	if err != nil && !errors.Is(err, context.Canceled) {
		logger.Errorw("error running web", "error", err)
		cancel()
		return err
	}

	return nil
}

func debugOut(ctx context.Context, r robot.Robot) {
	g, ok := r.InputControllerByName("gamepad1")
	if !ok {
		return
	}

	// repFunc := func(ctx context.Context, input input.Input, event input.Event) {
	// 	fmt.Printf("%s: %.4f\n", event.Code, event.Value)
	// 	return
	// }

	inputs, err := g.Inputs(ctx)
	if err != nil {
		return
	}

	for _, v := range inputs {

		state, _ := v.State(ctx)
		fmt.Printf("%s: %.4f\n", state.Code, state.Value)

		// err = v.RegisterControl(ctx, repFunc, input.AllEvents)
		// if err != nil {
		// 	return
		// }
	}

	// Loop forever
	for {
		time.Sleep(1000 * time.Millisecond)
	}

}
