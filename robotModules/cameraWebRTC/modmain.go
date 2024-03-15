package main

import (
	"context"

	goutils "go.viam.com/utils"

	"go.viam.com/rdk/components/camera"
	"go.viam.com/rdk/components/camera/fake"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/module"
	"go.viam.com/rdk/resource"
)

var Model = resource.NewModel("dang", "camera", "fake")

func newFakeCamera(
	ctx context.Context,
	deps resource.Dependencies,
	conf resource.Config,
	logger logging.Logger,
) (camera.Camera, error) {
	return fake.NewCamera(ctx, deps, conf, logger)
}

func mainWithArgs(ctx context.Context, args []string, logger logging.Logger) (err error) {
	resource.RegisterComponent(
		camera.API,
		Model,
		resource.Registration[camera.Camera, *fake.Config]{Constructor: newFakeCamera})

	module, err := module.NewModuleFromArgs(ctx, logger)
	if err != nil {
		return err
	}
	module.AddModelFromRegistry(ctx, camera.API, Model)

	err = module.Start(ctx)
	defer module.Close(ctx)
	if err != nil {
		return err
	}

	<-ctx.Done()
	return nil
}

func main() {
	goutils.ContextualMain(mainWithArgs, module.NewLoggerFromArgs("cameraWebRTC"))
}
