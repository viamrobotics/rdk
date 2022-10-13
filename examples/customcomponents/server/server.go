// Package main is a standalone server (for use as a remote) serving a demo Gizmo and Motor.
package main

import (
	"context"
	"errors"

	"github.com/edaniels/golog"
	goutils "go.viam.com/utils"

	"go.viam.com/rdk/config"
	_ "go.viam.com/rdk/examples/customcomponents/mygizmo"
	robotimpl "go.viam.com/rdk/robot/impl"
	"go.viam.com/rdk/robot/web"
)

var logger = golog.NewDebugLogger("gizmoserver")

func main() {
	goutils.ContextualMain(mainWithArgs, logger)
}

func mainWithArgs(ctx context.Context, args []string, logger golog.Logger) (err error) {
	if len(args) != 3 || args[1] != "-config" || len(args[2]) <= 0 {
		return errors.New("need -config option and valid json config file")
	}
	cfg, err := config.Read(ctx, args[2], logger)
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
