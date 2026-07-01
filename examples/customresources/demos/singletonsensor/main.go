// Package main is a singleton-on-disk sensor used by
// TestOptionalReconfigureFailureAndRecovery. First construction writes a
// lock file; the next construction (the one fired by the weak/optional
// reconfigure path) finds the lock and is refused, but clears it on the
// way out so the following worker tick can recover.
package main

import (
	"context"
	"errors"
	"os"
	"sync/atomic"

	"braces.dev/errtrace"
	"go.viam.com/rdk/components/sensor"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/module"
	"go.viam.com/rdk/resource"
)

var model = resource.NewModel("viam-test", "demo", "singleton-sensor")

type config struct {
	LockPath string `json:"lock_path"`
}

func (cfg *config) Validate(path string) ([]string, []string, error) {
	if cfg.LockPath == "" {
		return nil, nil, errtrace.Wrap(errors.New("lock_path required"))
	}
	return nil, []string{sensor.Named("opt").String()}, nil
}

type singletonSensor struct {
	resource.Named
	resource.AlwaysRebuild
	closed atomic.Bool
}

// refused gates the bug pattern to a single failure; subsequent attempts
// succeed so the test can observe recovery.
var refused atomic.Bool

func main() {
	resource.RegisterComponent(sensor.API, model, resource.Registration[sensor.Sensor, *config]{
		Constructor: newSensor,
	})
	module.ModularMain(resource.APIModel{API: sensor.API, Model: model})
}

func newSensor(_ context.Context, _ resource.Dependencies, conf resource.Config, _ logging.Logger) (sensor.Sensor, error) {
	cfg, err := resource.NativeConfig[*config](conf)
	if err != nil {
		return nil, errtrace.Wrap(err)
	}
	if _, err := os.Stat(cfg.LockPath); err == nil {
		if rmErr := os.Remove(cfg.LockPath); rmErr != nil {
			return nil, errtrace.Wrap(rmErr)
		}
		if !refused.Swap(true) {
			return nil, errtrace.Wrap(errors.New("singleton-sensor: lock present at " + cfg.LockPath))
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, errtrace.Wrap(err)
	}
	f, err := os.Create(cfg.LockPath)
	if err != nil {
		return nil, errtrace.Wrap(err)
	}
	if err := f.Close(); err != nil {
		return nil, errtrace.Wrap(err)
	}
	return &singletonSensor{Named: conf.ResourceName().AsNamed()}, nil
}

func (s *singletonSensor) Close(context.Context) error {
	s.closed.Store(true)
	return nil
}

func (s *singletonSensor) Readings(context.Context, map[string]interface{}) (map[string]interface{}, error) {
	if s.closed.Load() {
		return nil, errtrace.Wrap(errors.New("sensor is closed"))
	}
	return map[string]interface{}{}, nil
}
