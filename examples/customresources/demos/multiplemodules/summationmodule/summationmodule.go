// Package main is a module, which serves the mysum custom model type in the customresources examples.
package main

import (
	"context"

	"go.viam.com/utils"

	"go.viam.com/rdk/examples/customresources/apis/summationapi"
	"go.viam.com/rdk/examples/customresources/models/mysum"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/module"
)

func main() {
	// NewLoggerFromArgs will create a logging.Logger at "DebugLevel" if
	// "--log-level=debug" is the third argument in os.Args and at "InfoLevel"
	// otherwise.
	utils.ContextualMain(mainWithArgs, module.NewLoggerFromArgs("summationmodule"))
}

func mainWithArgs(ctx context.Context, args []string, logger logging.Logger) (err error) {
	myMod, err := module.NewModuleFromArgs(ctx, logger)
	if err != nil {
		return err
	}

	// Models and APIs add helpers to the registry during their init().
	// They can then be added to the module here.
	err = myMod.AddModelFromRegistry(ctx, summationapi.API, mysum.Model)
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
