// Package merged implements a movementsensor combining movement data from other sensors
package merged

import (
	"context"
	"fmt"
	"math"
	"sync"

	"github.com/golang/geo/r3"
	geo "github.com/kellydunn/golang-geo"
	"go.uber.org/multierr"
	"golang.org/x/exp/maps"

	"go.viam.com/rdk/components/movementsensor"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/spatialmath"
)

const errStrAccuracy = "_accuracy_err"

var model = resource.DefaultModelFamily.WithModel("merged")

// Config is the config of the merged movement_sensor model.
type Config struct {
	Position           []string `json:"position,omitempty"`
	Orientation        []string `json:"orientation,omitempty"`
	CompassHeading     []string `json:"compass_heading,omitempty"`
	LinearVelocity     []string `json:"linear_velocity,omitempty"`
	AngularVelocity    []string `json:"angular_velocity,omitempty"`
	LinearAcceleration []string `json:"linear_acceleration,omitempty"`
}

// Validate validates the merged model's configuration.
func (cfg *Config) Validate(path string) ([]string, error) {
	var deps []string
	deps = append(deps, cfg.Position...)
	deps = append(deps, cfg.Orientation...)
	deps = append(deps, cfg.CompassHeading...)
	deps = append(deps, cfg.LinearVelocity...)
	deps = append(deps, cfg.AngularVelocity...)
	deps = append(deps, cfg.LinearAcceleration...)
	return deps, nil
}

type merged struct {
	resource.Named
	logger logging.Logger

	mu sync.Mutex

	ori     movementsensor.MovementSensor
	pos     movementsensor.MovementSensor
	compass movementsensor.MovementSensor
	linVel  movementsensor.MovementSensor
	angVel  movementsensor.MovementSensor
	linAcc  movementsensor.MovementSensor
}

func init() {
	resource.Register(
		movementsensor.API, model,
		resource.Registration[movementsensor.MovementSensor, *Config]{
			Constructor: newMergedModel,
		})
}

func newMergedModel(ctx context.Context, deps resource.Dependencies, conf resource.Config, logger logging.Logger) (
	movementsensor.MovementSensor, error,
) {
	m := merged{
		logger: logger,
		Named:  conf.ResourceName().AsNamed(),
	}

	if err := m.Reconfigure(ctx, deps, conf); err != nil {
		return nil, err
	}

	return &m, nil
}

func (m *merged) Reconfigure(ctx context.Context, deps resource.Dependencies, conf resource.Config) error {
	newConf, err := resource.NativeConfig[*Config](conf)
	if err != nil {
		return err
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	firstGoodSensorWithProperties := func(
		deps resource.Dependencies, names []string, logger logging.Logger,
		want *movementsensor.Properties, propname string,
	) (movementsensor.MovementSensor, error) {
		// check if the config names and dependencies have been passed at all
		if len(names) == 0 || deps == nil {
			return nil, nil
		}

		for _, name := range names {
			ms, err := movementsensor.FromDependencies(deps, name)
			msName := ms.Name().ShortName()
			if err != nil {
				logger.CDebugf(ctx, "error getting sensor %v from dependencies", msName)
				continue
			}

			props, err := ms.Properties(ctx, nil)
			if err != nil {
				logger.CDebugf(ctx, "error in getting sensor %v properties", msName)
				continue
			}

			// check that the sensor matches the properties passed in
			// if it doesn't, skip it and go on to the next sensor in the list
			if want.OrientationSupported && !props.OrientationSupported {
				continue
			}

			if want.PositionSupported && !props.PositionSupported {
				continue
			}

			if want.CompassHeadingSupported && !props.CompassHeadingSupported {
				continue
			}

			if want.LinearVelocitySupported && !props.LinearVelocitySupported {
				continue
			}

			if want.AngularVelocitySupported && !props.AngularVelocitySupported {
				continue
			}

			if want.LinearAccelerationSupported && !props.LinearAccelerationSupported {
				continue
			}

			// we've found the sensor that reports everything we want
			m.logger.Debugf("using sensor %v as %s sensor", msName, propname)
			return ms, nil
		}

		return nil, fmt.Errorf("%v not supported by any sensor in list %#v", propname, names)
	}

	m.ori, err = firstGoodSensorWithProperties(
		deps, newConf.Orientation, m.logger,
		&movementsensor.Properties{OrientationSupported: true}, "orientation")
	if err != nil {
		return err
	}

	m.pos, err = firstGoodSensorWithProperties(
		deps, newConf.Position, m.logger,
		&movementsensor.Properties{PositionSupported: true}, "position")
	if err != nil {
		return err
	}

	m.compass, err = firstGoodSensorWithProperties(
		deps, newConf.CompassHeading, m.logger,
		&movementsensor.Properties{CompassHeadingSupported: true}, "compass_heading")
	if err != nil {
		return err
	}

	m.linVel, err = firstGoodSensorWithProperties(
		deps, newConf.LinearVelocity, m.logger,
		&movementsensor.Properties{LinearVelocitySupported: true}, "linear_velocity")
	if err != nil {
		return err
	}

	m.angVel, err = firstGoodSensorWithProperties(
		deps, newConf.AngularVelocity, m.logger,
		&movementsensor.Properties{AngularVelocitySupported: true}, "angular_velocity")
	if err != nil {
		return err
	}

	m.linAcc, err = firstGoodSensorWithProperties(
		deps, newConf.LinearAcceleration, m.logger,
		&movementsensor.Properties{LinearAccelerationSupported: true}, "linear_acceleration")
	if err != nil {
		return err
	}

	return nil
}

func (m *merged) Position(ctx context.Context, extra map[string]interface{}) (*geo.Point, float64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.pos == nil {
		return geo.NewPoint(math.NaN(), math.NaN()), math.NaN(),
			movementsensor.ErrMethodUnimplementedPosition
	}
	return m.pos.Position(ctx, extra)
}

func (m *merged) Orientation(ctx context.Context, extra map[string]interface{}) (spatialmath.Orientation, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.ori == nil {
		nanOri := spatialmath.NewOrientationVector()
		nanOri.OX = math.NaN()
		nanOri.OY = math.NaN()
		nanOri.OZ = math.NaN()
		nanOri.Theta = math.NaN()
		return nanOri,
			movementsensor.ErrMethodUnimplementedOrientation
	}
	return m.ori.Orientation(ctx, extra)
}

func (m *merged) CompassHeading(ctx context.Context, extra map[string]interface{}) (float64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.compass == nil {
		return math.NaN(),
			movementsensor.ErrMethodUnimplementedCompassHeading
	}
	return m.compass.CompassHeading(ctx, extra)
}

func (m *merged) LinearVelocity(ctx context.Context, extra map[string]interface{}) (r3.Vector, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.linVel == nil {
		return r3.Vector{X: math.NaN(), Y: math.NaN(), Z: math.NaN()},
			movementsensor.ErrMethodUnimplementedLinearVelocity
	}
	return m.linVel.LinearVelocity(ctx, extra)
}

func (m *merged) AngularVelocity(ctx context.Context, extra map[string]interface{}) (spatialmath.AngularVelocity, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.angVel == nil {
		return spatialmath.AngularVelocity{X: math.NaN(), Y: math.NaN(), Z: math.NaN()},
			movementsensor.ErrMethodUnimplementedAngularVelocity
	}
	return m.angVel.AngularVelocity(ctx, extra)
}

func (m *merged) LinearAcceleration(ctx context.Context, extra map[string]interface{}) (r3.Vector, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.linAcc == nil {
		return r3.Vector{X: math.NaN(), Y: math.NaN(), Z: math.NaN()},
			movementsensor.ErrMethodUnimplementedLinearAcceleration
	}
	return m.linAcc.LinearAcceleration(ctx, extra)
}

func mapWithSensorName(name string, accMap map[string]float32) map[string]float32 {
	result := map[string]float32{}
	for k, v := range accMap {
		result[name+"_"+k] = v
	}
	return result
}

func (m *merged) Accuracy(ctx context.Context, extra map[string]interface{}) (map[string]float32,
	float32, float32, movementsensor.NmeaGGAFixType, float32, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	accMap := make(map[string]float32)
	var errs error

	if m.ori != nil {
		oriAcc, _, _, _, _, err := m.ori.Accuracy(ctx, extra)
		if err != nil {
			// replace entire map with a map that shows that it has errors
			oriAcc = map[string]float32{
				m.ori.Name().ShortName() + errStrAccuracy: float32(math.NaN()),
			}
			errs = multierr.Combine(errs, err)
		}
		maps.Copy(accMap, mapWithSensorName(m.ori.Name().ShortName(), oriAcc))
	}

	if m.pos != nil {
		posAcc, _, _, _, _, err := m.pos.Accuracy(ctx, extra)
		if err != nil {
			posAcc = map[string]float32{
				m.pos.Name().ShortName() + errStrAccuracy: float32(math.NaN()),
			}
			errs = multierr.Combine(errs, err)
		}
		maps.Copy(accMap, mapWithSensorName(m.pos.Name().ShortName(), posAcc))
	}

	if m.compass != nil {
		compassAcc, _, _, _, _, err := m.compass.Accuracy(ctx, extra)
		if err != nil {
			compassAcc = map[string]float32{
				m.compass.Name().ShortName() + errStrAccuracy: float32(math.NaN()),
			}
			errs = multierr.Combine(errs, err)
		}
		maps.Copy(accMap, mapWithSensorName(m.compass.Name().ShortName(), compassAcc))
	}

	if m.linVel != nil {
		linvelAcc, _, _, _, _, err := m.linVel.Accuracy(ctx, extra)
		if err != nil {
			linvelAcc = map[string]float32{
				m.linVel.Name().ShortName() + errStrAccuracy: float32(math.NaN()),
			}
			errs = multierr.Combine(errs, err)
		}
		maps.Copy(accMap, mapWithSensorName(m.linVel.Name().ShortName(), linvelAcc))
	}

	if m.angVel != nil {
		angvelAcc, _, _, _, _, err := m.angVel.Accuracy(ctx, extra)
		if err != nil {
			angvelAcc = map[string]float32{
				m.angVel.Name().ShortName() + errStrAccuracy: float32(math.NaN()),
			}
			errs = multierr.Combine(errs, err)
		}
		maps.Copy(accMap, mapWithSensorName(m.angVel.Name().ShortName(), angvelAcc))
	}

	if m.linAcc != nil {
		linaccAcc, _, _, _, _, err := m.linAcc.Accuracy(ctx, extra)
		if err != nil {
			linaccAcc = map[string]float32{
				m.linAcc.Name().ShortName() + errStrAccuracy: float32(math.NaN()),
			}
			errs = multierr.Combine(errs, err)
		}
		maps.Copy(accMap, mapWithSensorName(m.linAcc.Name().ShortName(), linaccAcc))
	}

	return accMap, 0, 0, -1, 0, errs
}

func (m *merged) Properties(ctx context.Context, extra map[string]interface{}) (*movementsensor.Properties, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	return &movementsensor.Properties{
		PositionSupported:           m.pos != nil,
		OrientationSupported:        m.ori != nil,
		CompassHeadingSupported:     m.compass != nil,
		LinearVelocitySupported:     m.linVel != nil,
		AngularVelocitySupported:    m.angVel != nil,
		LinearAccelerationSupported: m.linAcc != nil,
	}, nil
}

func (m *merged) Readings(ctx context.Context, extra map[string]interface{}) (map[string]interface{}, error) {
	// we're already in lock in this driver
	// don't lock the mutex again for the Readings call
	return movementsensor.DefaultAPIReadings(ctx, m, extra)
}

func (m *merged) Close(context.Context) error {
	// we do not try to Close the movement sensors that this driver depends on
	// we let their own drivers and modules close them
	return nil
}
