// Package main is a module that exposes a single sensor model used to
// reproduce the use-after-failed-reconfigure bug. The sensor:
//   - Refuses to construct twice over the same lock file (singleton on disk).
//   - Sets an atomic "closed" flag on Close that Readings returns as an error.
//   - Declares an optional dep on a sensor named "opt", so reconfiguring the
//     "opt" sensor fires updateWeakAndOptionalDependents on this resource.
package main

import (
	"context"
	"errors"
	"os"
	"sync/atomic"

	"go.viam.com/rdk/components/sensor"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/module"
	"go.viam.com/rdk/resource"
)

// Model identifies the singleton sensor in the registry.
var Model = resource.NewModel("viam-test", "demo", "singleton-sensor")

const optionalDepName = "opt"

// Config holds the on-disk path of the singleton sentinel.
type Config struct {
	LockPath string `json:"lock_path"`
}

// Validate requires a lock path and declares the optional dep that drives the trigger.
func (cfg *Config) Validate(path string) ([]string, []string, error) {
	if cfg.LockPath == "" {
		return nil, nil, errors.New("lock_path required")
	}
	return nil, []string{sensor.Named(optionalDepName).String()}, nil
}

type singletonSensor struct {
	resource.Named
	resource.AlwaysRebuild
	closed atomic.Bool
}

func main() {
	resource.RegisterComponent(sensor.API, Model, resource.Registration[sensor.Sensor, *Config]{
		Constructor: newSensor,
	})
	module.ModularMain(resource.APIModel{API: sensor.API, Model: Model})
}

func newSensor(_ context.Context, _ resource.Dependencies, conf resource.Config, _ logging.Logger) (sensor.Sensor, error) {
	cfg, err := resource.NativeConfig[*Config](conf)
	if err != nil {
		return nil, err
	}
	if _, err := os.Stat(cfg.LockPath); err == nil {
		return nil, errors.New("singleton-sensor: lock present at " + cfg.LockPath +
			"; previous instance did not release it")
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	f, err := os.Create(cfg.LockPath)
	if err != nil {
		return nil, err
	}
	if err := f.Close(); err != nil {
		return nil, err
	}
	return &singletonSensor{Named: conf.ResourceName().AsNamed()}, nil
}

func (s *singletonSensor) Close(context.Context) error {
	s.closed.Store(true)
	return nil
}

func (s *singletonSensor) Readings(context.Context, map[string]interface{}) (map[string]interface{}, error) {
	if s.closed.Load() {
		return nil, errors.New("sensor is closed!")
	}
	return map[string]interface{}{}, nil
}
