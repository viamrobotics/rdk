// Package main is a standalone server (for use as a remote) serving a demo Gizmo component.
package main

import (
	"context"
	"errors"

	"github.com/edaniels/golog"
	goutils "go.viam.com/utils"

	"go.viam.com/rdk/config"
	_ "go.viam.com/rdk/examples/customresources/models/mygizmo"
	robotimpl "go.viam.com/rdk/robot/impl"
	"go.viam.com/rdk/robot/web"
)

var logger = golog.NewDebugLogger("gizmoserver")

// Arguments for the command.
type Arguments struct {
	ConfigFile string `flag:"config,usage=robot config file"`
}

func main() {
	goutils.ContextualMain(mainWithArgs, logger)
}

func mainWithArgs(ctx context.Context, args []string, logger golog.Logger) (err error) {
	var argsParsed Arguments
	if err := goutils.ParseFlags(args, &argsParsed); err != nil {
		return err
	}

	if argsParsed.ConfigFile == "" {
		return errors.New("please specify a config file through the -config parameter.")
	}

	cfg, err := config.Read(ctx, argsParsed.ConfigFile, logger)
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
