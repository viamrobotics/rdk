// Package main shows a simple server with a fake arm.
package main

import (
	"context"

	"github.com/viamrobotics/gostream/codec/x264"
	"go.viam.com/utils"

	"go.viam.com/rdk/components/arm"
	"go.viam.com/rdk/components/arm/fake"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
	robotimpl "go.viam.com/rdk/robot/impl"
	"go.viam.com/rdk/robot/web"
)

var logger = logging.NewDebugLogger("simpleserver")

func main() {
	utils.ContextualMain(mainWithArgs, logger)
}

func mainWithArgs(ctx context.Context, args []string, logger logging.Logger) error {
	arm1Name := arm.Named("arm1")
	cfg := resource.Config{
		Name:  arm1Name.Name,
		Model: resource.DefaultModelFamily.WithModel("ur5e"),
		ConvertedAttributes: &fake.Config{
			ArmModel: "ur5e",
		},
	}
	arm1, err := fake.NewArm(context.Background(), nil, cfg, logger)
	if err != nil {
		return err
	}
	myRobot, err := robotimpl.RobotFromResources(
		ctx,
		map[resource.Name]resource.Resource{
			arm1Name: arm1,
		},
		logger,
		robotimpl.WithWebOptions(web.WithStreamConfig(x264.DefaultStreamConfig)),
	)
	if err != nil {
		return err
	}
	//nolint:errcheck
	defer myRobot.Close(ctx)

	<-ctx.Done()
	return nil
}
