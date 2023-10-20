// Package main is an example of a custom viam server.
package main

import (
	"context"

	goutils "go.viam.com/utils"

	"go.viam.com/rdk/components/sensor"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
	robotimpl "go.viam.com/rdk/robot/impl"
	"go.viam.com/rdk/robot/web"
	weboptions "go.viam.com/rdk/robot/web/options"
)

var logger = logging.NewDebugLogger("mysensor")

// registering the component model on init is how we make sure the new model is picked up and usable.
func init() {
	resource.RegisterComponent(
		sensor.API,
		resource.DefaultModelFamily.WithModel("mySensor"),
		resource.Registration[sensor.Sensor, resource.NoNativeConfig]{Constructor: func(
			ctx context.Context,
			deps resource.Dependencies,
			conf resource.Config,
			logger logging.ZapCompatibleLogger,
		) (sensor.Sensor, error) {
			return newSensor(conf.ResourceName()), nil
		}})
}

func newSensor(name resource.Name) sensor.Sensor {
	return &mySensor{
		Named: name.AsNamed(),
	}
}

// mySensor is a sensor device that always returns "hello world".
type mySensor struct {
	resource.Named
	resource.AlwaysRebuild
	resource.TriviallyCloseable
}

// Readings always returns "hello world".
func (s *mySensor) Readings(ctx context.Context, _ map[string]interface{}) (map[string]interface{}, error) {
	return map[string]interface{}{"hello": "world"}, nil
}

func main() {
	goutils.ContextualMain(mainWithArgs, logger)
}

func mainWithArgs(ctx context.Context, args []string, logger logging.Logger) (err error) {
	name := sensor.Named("sensor1")
	s := newSensor(name)

	myRobot, err := robotimpl.RobotFromResources(ctx, map[resource.Name]resource.Resource{name: s}, logger)
	if err != nil {
		return err
	}
	//nolint:errcheck
	defer myRobot.Close(ctx)
	o := weboptions.New()
	// the default bind address is localhost:8080, specifying a different bind address to avoid collisions.
	o.Network.BindAddress = "localhost:8081"

	// runs the web server on the robot and blocks until the program is stopped.
	return web.RunWeb(ctx, myRobot, o, logger)
}
