// Package merge implements a merge GPS that returns the first measurment of multiple GPS devices.
package merge

import (
	"context"

	"github.com/edaniels/golog"
	geo "github.com/kellydunn/golang-geo"
	"github.com/pkg/errors"
	"go.uber.org/multierr"

	"go.viam.com/rdk/component/generic"
	"go.viam.com/rdk/component/gps"
	"go.viam.com/rdk/component/sensor"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/robot"
)

// ModelName is the name of th merge model for gps.
const ModelName = "merge"

func init() {
	registry.RegisterComponent(
		gps.Subtype,
		ModelName, registry.Component{Constructor: func(
			ctx context.Context,
			r robot.Robot,
			config config.Component,
			logger golog.Logger,
		) (interface{}, error) {
			return newMerge(r, config, logger)
		}})
}

func newMerge(r robot.Robot, config config.Component, logger golog.Logger) (gps.LocalGPS, error) {
	subs := config.Attributes.StringSlice("subs")
	if len(subs) == 0 {
		return nil, errors.New("no subs for merge gps")
	}

	m := &mergeGPS{r, nil, logger, generic.Unimplemented{}}

	for _, s := range subs {
		g, err := gps.FromRobot(r, s)
		if err != nil {
			return nil, err
		}

		m.subs = append(m.subs, g)
	}
	return m, nil
}

type mergeGPS struct {
	r      robot.Robot
	subs   []gps.GPS
	logger golog.Logger
	generic.Unimplemented
}

// The current latitude and longitude.
func (m *mergeGPS) ReadLocation(ctx context.Context) (*geo.Point, error) {
	var allErrors error
	for _, g := range m.subs {
		p, err := g.ReadLocation(ctx)
		if err == nil {
			return p, nil
		}
		allErrors = multierr.Combine(allErrors, err)
	}
	return nil, allErrors
}

// The current altitude in meters.
func (m *mergeGPS) ReadAltitude(ctx context.Context) (float64, error) {
	var allErrors error
	for _, g := range m.subs {
		a, err := g.ReadAltitude(ctx)
		if err == nil {
			return a, nil
		}
		allErrors = multierr.Combine(allErrors, err)
	}
	return 0, allErrors
}

// Current ground speed in kph.
func (m *mergeGPS) ReadSpeed(ctx context.Context) (float64, error) {
	var allErrors error
	for _, g := range m.subs {
		s, err := g.ReadSpeed(ctx)
		if err == nil {
			return s, nil
		}
		allErrors = multierr.Combine(allErrors, err)
	}
	return 0, allErrors
}

// Number of satellites used for fix, and total in view.
func (m *mergeGPS) ReadSatellites(ctx context.Context) (int, int, error) {
	var allErrors error
	for _, g := range m.subs {
		localG, ok := g.(gps.LocalGPS)
		if !ok {
			continue
		}
		a, b, err := localG.ReadSatellites(ctx)
		if err == nil {
			return a, b, nil
		}
		allErrors = multierr.Combine(allErrors, err)
	}
	return 0, 0, allErrors
}

// Horizontal and vertical position error in meters.
func (m *mergeGPS) ReadAccuracy(ctx context.Context) (float64, float64, error) {
	var allErrors error
	for _, g := range m.subs {
		localG, ok := g.(gps.LocalGPS)
		if !ok {
			continue
		}
		a, b, err := localG.ReadAccuracy(ctx)
		if err == nil {
			return a, b, nil
		}
		allErrors = multierr.Combine(allErrors, err)
	}
	return 0, 0, allErrors
}

// Whether or not the GPS chip had a valid fix for the most recent dataset.
func (m *mergeGPS) ReadValid(ctx context.Context) (bool, error) {
	var allErrors error
	for _, g := range m.subs {
		localG, ok := g.(gps.LocalGPS)
		if !ok {
			continue
		}
		v, err := localG.ReadValid(ctx)
		if err == nil {
			return v, nil
		}
		allErrors = multierr.Combine(allErrors, err)
	}
	return false, allErrors
}

// GetReadings return data specific to the type of sensor and can be of any type.
func (m *mergeGPS) GetReadings(ctx context.Context) ([]interface{}, error) {
	var (
		r         []interface{}
		err       error
		allErrors error
	)
	for _, g := range m.subs {
		s, ok := g.(sensor.Sensor)
		if ok {
			r, err = s.GetReadings(ctx)
		} else {
			r, err = gps.GetReadings(ctx, g)
		}
		if err == nil {
			return r, nil
		}
		allErrors = multierr.Combine(allErrors, err)
	}
	return nil, allErrors
}
