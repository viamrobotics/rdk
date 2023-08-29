// Package sensorfilter implements a modular component that filters the output of an underlying ultrasonic sensor
// and only keeps captured data if there is a significant change in readings.
package sensorfilter

import (
	"context"
	"fmt"
	"math"

	"github.com/edaniels/golog"
	"github.com/pkg/errors"

	"go.viam.com/rdk/components/sensor"
	"go.viam.com/rdk/data"
	"go.viam.com/rdk/resource"
)

// Model is the full model definition.
var Model = resource.NewModel("example", "sensor", "sensorfilter")

func init() {
	resource.RegisterComponent(sensor.API, Model, resource.Registration[sensor.Sensor, *Config]{
		Constructor: newSensor,
	})
}

func newSensor(ctx context.Context, deps resource.Dependencies, conf resource.Config, logger golog.Logger) (sensor.Sensor, error) {
	s := &filterSensor{
		Named:  conf.ResourceName().AsNamed(),
		logger: logger,
	}
	if err := s.Reconfigure(ctx, deps, conf); err != nil {
		return nil, err
	}
	return s, nil
}

// Config contains the name to the underlying sensor.
type Config struct {
	ActualSensor string `json:"actual_sensor"`
}

// Validate validates the config and returns implicit dependencies.
func (cfg *Config) Validate(path string) ([]string, error) {
	if cfg.ActualSensor == "" {
		return nil, fmt.Errorf(`expected "actual_sensor" attribute in %q`, path)
	}

	return []string{cfg.ActualSensor}, nil
}

// A filterSensor wraps the underlying sensor `actualSensor` and only keeps the data captured on the actual sensor if the current reading
// is significantly different from the previously captured reading.
type filterSensor struct {
	resource.Named
	actualSensor sensor.Sensor
	prevReadings map[string]interface{}
	logger       golog.Logger
}

// Reconfigure reconfigures the modular component with new settings.
func (s *filterSensor) Reconfigure(ctx context.Context, deps resource.Dependencies, conf resource.Config) error {
	sensorConfig, err := resource.NativeConfig[*Config](conf)
	if err != nil {
		return err
	}

	s.actualSensor, err = sensor.FromDependencies(deps, sensorConfig.ActualSensor)
	if err != nil {
		return errors.Wrapf(err, "unable to get sensor %v for sensorfilter", sensorConfig.ActualSensor)
	}
	return nil
}

// DoCommand simply echoes whatever was sent.
func (s *filterSensor) DoCommand(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	return cmd, nil
}

func delta(curr, prev map[string]interface{}, logger golog.Logger) float64 {
	currDist, ok := curr["distance"].(float64)
	if !ok {
		logger.Errorw("sensor's current distance reading is not of type float", "currReading", curr)
		return 0
	}
	prevDist, ok := prev["distance"].(float64)
	if !ok {
		logger.Errorw("sensor's previous distance reading is not of type float", "prevReading", prev)
		return 0
	}
	diff := currDist - prevDist
	return math.Abs(diff / prevDist)
}

// Readings returns data from the sensor.
func (s *filterSensor) Readings(ctx context.Context, extra map[string]interface{}) (map[string]interface{}, error) {
	if extra[data.FromDMString] != true {
		// If not data management collector, return underlying sensor contents without filtering.
		return s.actualSensor.Readings(ctx, extra)
	}
	readings, err := s.actualSensor.Readings(ctx, extra)
	if err != nil {
		return nil, errors.Wrap(err, "could not get next reading from sensor")
	}

	// Only return captured readings if they are significantly different from the previously stored readings.
	if s.prevReadings == nil || delta(readings, s.prevReadings, s.logger) > 0.1 {
		s.prevReadings = readings
		return readings, nil
	}

	return nil, data.ErrNoCaptureToStore
}

// Close closes the underlying sensor.
func (s *filterSensor) Close(ctx context.Context) error {
	return s.actualSensor.Close(ctx)
}
