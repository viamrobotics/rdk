// Package main is a module, which serves all four custom model types in the customresources examples, including both custom APIs.
package main

import (
	"context"

	"github.com/edaniels/golog"

	"go.viam.com/rdk/components/base"
	"go.viam.com/rdk/module"
	"go.viam.com/rdk/services/navigation"

	"go.viam.com/rdk/examples/customresources/apis/gizmoapi"
	"go.viam.com/rdk/examples/customresources/apis/summationapi"
	"go.viam.com/rdk/examples/customresources/models/mybase"
	"go.viam.com/rdk/examples/customresources/models/mygizmo"
	"go.viam.com/rdk/examples/customresources/models/mynavigation"
	"go.viam.com/rdk/examples/customresources/models/mysum"

	"go.viam.com/utils"
)

func main() {
	utils.ContextualMain(mainWithArgs, golog.NewDevelopmentLogger("ComplexModule"))
}

func mainWithArgs(ctx context.Context, args []string, logger golog.Logger) (err error) {
	myMod, err := module.NewModuleFromArgs(ctx, logger)
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
	defer myMod.Close(ctx)
	if err != nil {
		return err
	}
	<-ctx.Done()
	return nil
}
