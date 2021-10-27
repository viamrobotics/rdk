package merge

import (
	"context"
	"errors"
	"fmt"

	"github.com/edaniels/golog"
	geo "github.com/kellydunn/golang-geo"
	"go.uber.org/multierr"

	"go.viam.com/core/config"
	"go.viam.com/core/registry"
	"go.viam.com/core/robot"
	"go.viam.com/core/sensor"
	"go.viam.com/core/sensor/gps"
)

func init() {
	registry.RegisterSensor(gps.Type, "merge", registry.Sensor{Constructor: func(ctx context.Context, r robot.Robot, config config.Component, logger golog.Logger) (sensor.Sensor, error) {
		return newMerge(r, config, logger)
	}})
}

func newMerge(r robot.Robot, config config.Component, logger golog.Logger) (gps.GPS, error) {
	subs := config.Attributes.StringSlice("subs")
	if len(subs) == 0 {
		return nil, errors.New("no subs for merge gps")
	}

	m := &mergeGPS{r, nil, logger}

	for _, s := range subs {
		sensor, ok := r.SensorByName(s)
		if !ok {
			return nil, fmt.Errorf("no gps named [%s]", s)
		}

		g, ok := sensor.(gps.GPS)
		if !ok {
			return nil, fmt.Errorf("sensor named [%s] is not a gps, is a %T", s, sensor)
		}

		m.subs = append(m.subs, g)
	}
	return m, nil
}

type mergeGPS struct {
	r      robot.Robot
	subs   []gps.GPS
	logger golog.Logger
}

// The current latitude and longitude
func (m *mergeGPS) Location(ctx context.Context) (*geo.Point, error) {
	var bigError error
	for _, g := range m.subs {
		p, err := g.Location(ctx)
		if err == nil {
			return p, nil
		}
		bigError = multierr.Combine(bigError, err)
	}
	return nil, bigError
}

// The current altitude in meters
func (m *mergeGPS) Altitude(ctx context.Context) (float64, error) {
	var bigError error
	for _, g := range m.subs {
		a, err := g.Altitude(ctx)
		if err == nil {
			return a, nil
		}
		bigError = multierr.Combine(bigError, err)
	}
	return 0, bigError
}

// Current ground speed in kph
func (m *mergeGPS) Speed(ctx context.Context) (float64, error) {
	var bigError error
	for _, g := range m.subs {
		s, err := g.Speed(ctx)
		if err == nil {
			return s, nil
		}
		bigError = multierr.Combine(bigError, err)
	}
	return 0, bigError
}

// Number of satellites used for fix, and total in view
func (m *mergeGPS) Satellites(ctx context.Context) (int, int, error) {
	var bigError error
	for _, g := range m.subs {
		a, b, err := g.Satellites(ctx)
		if err == nil {
			return a, b, nil
		}
		bigError = multierr.Combine(bigError, err)
	}
	return 0, 0, bigError
}

// Horizontal and vertical position error
func (m *mergeGPS) Accuracy(ctx context.Context) (float64, float64, error) {
	var bigError error
	for _, g := range m.subs {
		a, b, err := g.Accuracy(ctx)
		if err == nil {
			return a, b, nil
		}
		bigError = multierr.Combine(bigError, err)
	}
	return 0, 0, bigError
}

// Whether or not the GPS chip had a valid fix for the most recent dataset
func (m *mergeGPS) Valid(ctx context.Context) (bool, error) {
	var bigError error
	for _, g := range m.subs {
		v, err := g.Valid(ctx)
		if err == nil {
			return v, nil
		}
		bigError = multierr.Combine(bigError, err)
	}
	return false, bigError
}

// Readings return data specific to the type of sensor and can be of any type.
func (m *mergeGPS) Readings(ctx context.Context) ([]interface{}, error) {
	var bigError error
	for _, g := range m.subs {
		r, err := g.Readings(ctx)
		if err == nil {
			return r, nil
		}
		bigError = multierr.Combine(bigError, err)
	}
	return nil, bigError
}

// Desc returns a description of this sensor.
func (m *mergeGPS) Desc() sensor.Description {
	return sensor.Description{gps.Type, ""}
}
