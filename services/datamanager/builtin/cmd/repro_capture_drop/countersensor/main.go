// Package main is a module that implements a sensor with a monotonically
// incrementing counter. Each call to Readings returns {"count": N} where N
// increases by 1. This makes it possible to detect dropped data: if the
// captured sequence is 1, 2, 3, 5 then reading 4 was polled but never
// written to disk.
package main

import (
	"context"
	"sync/atomic"

	"go.viam.com/rdk/components/sensor"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/module"
	"go.viam.com/rdk/resource"
)

var model = resource.NewModel("repro", "test", "countersensor")

func main() {
	resource.RegisterComponent(sensor.API, model, resource.Registration[resource.Resource, resource.NoNativeConfig]{
		Constructor: newCounterSensor,
	})
	module.ModularMain(resource.APIModel{API: sensor.API, Model: model})
}

func newCounterSensor(
	ctx context.Context,
	deps resource.Dependencies,
	conf resource.Config,
	logger logging.Logger,
) (resource.Resource, error) {
	return &counterSensor{
		Named:  conf.ResourceName().AsNamed(),
		logger: logger,
	}, nil
}

type counterSensor struct {
	resource.Named
	resource.TriviallyCloseable
	resource.TriviallyReconfigurable
	logger logging.Logger
	count  atomic.Int64
}

func (c *counterSensor) Readings(ctx context.Context, extra map[string]interface{}) (map[string]interface{}, error) {
	n := c.count.Add(1)
	return map[string]interface{}{"count": n}, nil
}

func (c *counterSensor) DoCommand(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	return nil, nil
}
