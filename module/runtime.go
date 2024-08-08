package module

import (
	"context"

	"go.viam.com/utils"

	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
)

// ModularMain can be called as the main function from a module. It will start up a module with all
// the provided APIModels added to it.
func ModularMain(moduleName string, models ...resource.APIModel) {
	mainWithArgs := func(ctx context.Context, args []string, logger logging.Logger) error {
		mod, err := NewModuleFromArgs(ctx, logger)
		if err != nil {
			return err
		}

		for _, apiModel := range models {
			if err = mod.AddModelFromRegistry(ctx, apiModel.API, apiModel.Model); err != nil {
				return err
			}
		}

		err = mod.Start(ctx)
		defer mod.Close(ctx)
		if err != nil {
			return err
		}

		<-ctx.Done()
		return nil
	}

	utils.ContextualMain(mainWithArgs, NewLoggerFromArgs(moduleName))
}
