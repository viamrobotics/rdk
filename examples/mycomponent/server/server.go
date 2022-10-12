// Package main is a sample of a user defined and implemented component type
package main

import (
	"context"

	"github.com/edaniels/golog"
	goutils "go.viam.com/utils"

	"go.viam.com/rdk/config"
	_ "go.viam.com/rdk/examples/mycomponent/component"
	robotimpl "go.viam.com/rdk/robot/impl"
	"go.viam.com/rdk/robot/web"
	"go.viam.com/rdk/utils"
)

var logger = golog.NewDebugLogger("mycomponent")

func main() {
	goutils.ContextualMain(mainWithArgs, logger)
}

func mainWithArgs(ctx context.Context, args []string, logger golog.Logger) (err error) {
	cfg, err := config.Read(ctx, utils.ResolveFile("./examples/mycomponent/server/config.json"), logger)
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
