// Package main is a standalone server (for use as a remote) serving a demo Gizmo component.
package main

import (
	"context"
	"errors"

	goutils "go.viam.com/utils"
	"go.viam.com/utils/rpc"

	"go.viam.com/rdk/config"
	_ "go.viam.com/rdk/examples/customresources/models/mygizmo"
	"go.viam.com/rdk/grpc"
	"go.viam.com/rdk/logging"
	robotimpl "go.viam.com/rdk/robot/impl"
	"go.viam.com/rdk/robot/web"
)

var logger = logging.NewDebugLogger("gizmoserver")

// Arguments for the command.
type Arguments struct {
	ConfigFile string `flag:"config,usage=machine config file"`
}

func main() {
	goutils.ContextualMain(mainWithArgs, logger)
}

func mainWithArgs(ctx context.Context, args []string, logger logging.Logger) (err error) {
	var argsParsed Arguments
	if err := goutils.ParseFlags(args, &argsParsed); err != nil {
		return err
	}

	if argsParsed.ConfigFile == "" {
		return errors.New("please specify a config file through the -config parameter")
	}

	cfg, err := config.Read(ctx, argsParsed.ConfigFile, logger, nil)
	if err != nil {
		return err
	}

	var appConn rpc.ClientConn
	if cfg.Cloud != nil && cfg.Cloud.AppAddress != "" {
		authID, _, authSecret := cfg.Cloud.GetAuthCredentials()
		appConn, err = grpc.NewAppConn(
			ctx, cfg.Cloud.AppAddress, cfg.Cloud.ID, authID, authSecret, logger)
		if err != nil {
			return nil
		}

		defer goutils.UncheckedErrorFunc(appConn.Close)
	}

	myRobot, err := robotimpl.RobotFromConfig(ctx, cfg, appConn, logger)
	if err != nil {
		return err
	}
	//nolint:errcheck
	defer myRobot.Close(ctx)

	return web.RunWebWithConfig(ctx, myRobot, cfg, logger)
}
