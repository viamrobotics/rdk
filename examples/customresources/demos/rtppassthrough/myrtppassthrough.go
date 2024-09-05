// Package main implements a fake passthrough camera module
package main

import (
	"context"

	"go.viam.com/rdk/components/camera"
	"go.viam.com/rdk/components/camera/fake"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/module"
	"go.viam.com/rdk/resource"
)

var model = resource.NewModel("acme", "camera", "fake")

func main() {
	resource.RegisterComponent(
		camera.API,
		model,
		resource.Registration[camera.Camera, *fake.Config]{Constructor: newFakeCamera})

	module.ModularMain("rtp-passthrough-camera", resource.APIModel{camera.API, model})
}

func newFakeCamera(
	ctx context.Context,
	deps resource.Dependencies,
	conf resource.Config,
	logger logging.Logger,
) (camera.Camera, error) {
	return fake.NewCamera(ctx, deps, conf, logger)
}
