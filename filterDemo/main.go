// Package main is a module which serves the katcam custom model.
package main

import (
	"context"

	"github.com/edaniels/golog"

	"go.viam.com/rdk/components/camera"
	"go.viam.com/rdk/module"
	"go.viam.com/utils"

	"go.viam.com/rdk/filterDemo/katcam"
)

func main() {
	utils.ContextualMain(mainWithArgs, module.NewLoggerFromArgs("katlog_module"))
}

func mainWithArgs(ctx context.Context, args []string, logger golog.Logger) (err error) {
	myMod, err := module.NewModuleFromArgs(ctx, logger)
	if err != nil {
		return err
	}

	err = myMod.AddModelFromRegistry(ctx, camera.API, katcam.Model)
	if err != nil {
		return err
	}

	err = myMod.Start(ctx)
	defer myMod.Close(ctx)
	if err != nil {
		return err
	}
	<-ctx.Done()
	return nil
}
