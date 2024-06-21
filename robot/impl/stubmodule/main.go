// Package main is a module which serves the customsensor custom model.
package main

import (
	"context"
	"errors"

	"go.viam.com/rdk/components/sensor"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/module"
	"go.viam.com/rdk/resource"
	"go.viam.com/utils"
)

var (
	Model = resource.NewModel("viam", "test-suite", "stubmodule")
)

type Config struct{}

func (cfg *Config) Validate(path string) ([]string, error) {
	return []string{}, nil
}

func init() {
	resource.RegisterComponent(sensor.API, Model,
		resource.Registration[sensor.Sensor, *Config]{
			Constructor: newSensor,
		},
	)
}

func main() {
	utils.ContextualMain(mainWithArgs, module.NewLoggerFromArgs("stubmodule"))
}

func mainWithArgs(ctx context.Context, args []string, logger logging.Logger) (err error) {
	myModule, err := module.NewModuleFromArgs(ctx, logger)
	if err != nil {
		return err
	}

	err = myModule.AddModelFromRegistry(ctx, sensor.API, Model)
	if err != nil {
		return err
	}

	err = myModule.Start(ctx)
	defer myModule.Close(ctx)
	if err != nil {
		return err
	}
	<-ctx.Done()
	return nil
}

type stubSensor struct {
	resource.Named
	resource.TriviallyReconfigurable
	resource.TriviallyCloseable
}

func (stubSensor) Readings(ctx context.Context, extra map[string]any) (map[string]any, error) {
	return nil, errors.New("notImplemented")
}

func newSensor(ctx context.Context, deps resource.Dependencies, rawConf resource.Config, logger logging.Logger) (sensor.Sensor, error) {
	return stubSensor{}, nil
}
