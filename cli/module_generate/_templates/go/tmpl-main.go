package main

import (
	"{{.ModuleName}}/models"
	"context"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/module"
	"go.viam.com/utils"
	"go.viam.com/rdk/{{.ResourceType}}s/{{.ResourceSubtype}}"

)

func main() {
	utils.ContextualMain(mainWithArgs, module.NewLoggerFromArgs("{{.ModuleName}}"))
}

func mainWithArgs(ctx context.Context, args []string, logger logging.Logger) error {
	{{.ModuleCamel}}, err := module.NewModuleFromArgs(ctx)
	if err != nil {
		return err
	}
	
	if err = {{.ModuleCamel}}.AddModelFromRegistry(ctx, {{.ResourceSubtype}}.API, models.{{.ModelPascal}}); err != nil {
		return err
	}


	err = {{.ModuleCamel}}.Start(ctx)
	defer {{.ModuleCamel}}.Close(ctx)
	if err != nil {
		return err
	}

	<-ctx.Done()
	return nil
}
