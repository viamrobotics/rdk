// Package main is an example of a custom viam server.
package main

import (
	"context"

	"github.com/edaniels/golog"
	goutils "go.viam.com/utils"

	"go.viam.com/rdk/config"
	// import the custom sensor package to register it properly
	_ "go.viam.com/rdk/examples/mysensor/mysensor"
	robotimpl "go.viam.com/rdk/robot/impl"
	"go.viam.com/rdk/robot/web"
	"go.viam.com/rdk/utils"
)

var logger = golog.NewDevelopmentLogger("mysensor")

func main() {
	goutils.ContextualMain(mainWithArgs, logger)
}

func mainWithArgs(ctx context.Context, args []string, logger golog.Logger) (err error) {
	// we set the config here, but it can also be a commandline argument if so desired.
	cfg, err := config.Read(ctx, utils.ResolveFile("./examples/mysensor/server/config.json"), logger)
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
