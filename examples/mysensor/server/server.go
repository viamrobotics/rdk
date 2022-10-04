// Package main is an example of a custom viam server.
package main

import (
	"context"

	"github.com/edaniels/golog"
	goutils "go.viam.com/utils"

	"go.viam.com/rdk/components/generic"
	"go.viam.com/rdk/components/sensor"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/registry"

	robotimpl "go.viam.com/rdk/robot/impl"
	"go.viam.com/rdk/robot/web"
	"go.viam.com/rdk/utils"
)

var logger = golog.NewDebugLogger("mysensor")

// registering the component model on init is how we make sure the new model is picked up and usable.
func init() {
	registry.RegisterComponent(
		sensor.Subtype,
		"mySensor",
		registry.Component{Constructor: func(
			ctx context.Context,
			deps registry.Dependencies,
			config config.Component,
			logger golog.Logger,
		) (interface{}, error) {
			return newSensor(config.Name), nil
		}})
}

func newSensor(name string) sensor.Sensor {
	return &mySensor{Name: name}
}

// this checks that the mySensor struct implements the sensor.Sensor interface.
var _ = sensor.Sensor(&mySensor{})

// mySensor is a sensor device that always returns "hello world".
type mySensor struct {
	Name string

	// generic.Unimplemented is a helper that embeds an unimplemented error in the Do method.
	generic.Unimplemented
}

// Readings always returns "hello world".
func (s *mySensor) Readings(ctx context.Context) (map[string]interface{}, error) {
	return map[string]interface{}{"hello": "world"}, nil
}

func main() {
	goutils.ContextualMain(mainWithArgs, logger)
}

func mainWithArgs(ctx context.Context, args []string, logger golog.Logger) (err error) {
	// we set the config here, but it can also be a commandline argument if so desired.
	cfg, err := config.Read(ctx, utils.ResolveFile("./examples/mysensor/server/config.json"), logger)
	if err != nil {
		return err
	}
	myRobot, err := robotimpl.RobotFromConfig(ctx, cfg, logger)
	if err != nil {
		return err
	}
	defer myRobot.Close(ctx)

	return web.RunWebWithConfig(ctx, myRobot, cfg, logger)
}
