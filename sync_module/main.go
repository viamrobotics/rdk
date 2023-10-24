// Package main is a module which serves the selective sync custom model.
package main

import (
	"context"

	"github.com/edaniels/golog"
	"go.viam.com/utils"

	"go.viam.com/rdk/components/generic"
	"go.viam.com/rdk/module"
	selectiveSync "go.viam.com/rdk/sync_module/selective_sync"
)

func main() {
	utils.ContextualMain(mainWithArgs, module.NewLoggerFromArgs("selective_sync_module"))
}

func mainWithArgs(ctx context.Context, args []string, logger golog.Logger) (err error) {
	myMod, err := module.NewModuleFromArgs(ctx, logger)
	if err != nil {
		return err
	}

	err = myMod.AddModelFromRegistry(ctx, generic.API, selectiveSync.Model)
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
