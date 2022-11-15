package main

import (
	"context"

	"github.com/edaniels/golog"
	"github.com/pkg/errors"
	"go.uber.org/zap"

	"go.viam.com/rdk/components/base"
	"go.viam.com/rdk/module"
	"go.viam.com/rdk/services/navigation"

	"go.viam.com/rdk/examples/customresources/gizmoapi"
	"go.viam.com/rdk/examples/customresources/mynavigation"
	"go.viam.com/rdk/examples/customresources/summationapi"

	"go.viam.com/rdk/examples/customresources/mybase"
	"go.viam.com/rdk/examples/customresources/mygizmo"
	"go.viam.com/rdk/examples/customresources/mysum"

	"go.viam.com/utils"
)

var logger = NewLogger()

func NewLogger() (*zap.SugaredLogger) {
	cfg := zap.NewDevelopmentConfig()
	cfg.OutputPaths = []string{"/tmp/mod.log"}
	l, err := cfg.Build()
	if err != nil {
		return nil
	}
	return l.Sugar()
}

func main() {
	utils.ContextualMain(mainWithArgs, logger.Named("acme demo module"))
}

func mainWithArgs(ctx context.Context, args []string, logger golog.Logger) (err error) {
	if len(args) < 2 {
		return errors.New("need socket path as command line argument")
	}
	socketPath := args[1]

	myMod, err := module.NewModule(ctx, socketPath, logger)
	if err != nil {
		return err
	}

	// Models and APIs add helpers to the registry during their init().
	// They can then be added to the module here.
	myMod.AddModelFromRegistry(ctx, gizmoapi.Subtype, mygizmo.Model)
	myMod.AddModelFromRegistry(ctx, summationapi.Subtype, mysum.Model)
	myMod.AddModelFromRegistry(ctx, base.Subtype, mybase.Model)
	myMod.AddModelFromRegistry(ctx, navigation.Subtype, mynavigation.Model)

	err = myMod.Start(ctx)
	defer myMod.Close()
	if err != nil {
		return err
	}
	<-ctx.Done()
	return nil
}
