// Package fake implements a fake GPS.
package fake

import (
	"context"
	"sync"

	"github.com/edaniels/golog"
	geo "github.com/kellydunn/golang-geo"
	"github.com/pkg/errors"

	"go.viam.com/rdk/component/base"
	"go.viam.com/rdk/component/gps"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/robot"
	"go.viam.com/rdk/utils"
)

func init() {
	registry.RegisterComponent(
		gps.Subtype,
		"fake",
		registry.Component{Constructor: func(
			ctx context.Context,
			r robot.Robot,
			config config.Component,
			logger golog.Logger,
		) (interface{}, error) {
			return &GPS{Name: config.Name}, nil
		}})
	registry.RegisterComponent(
		base.Subtype,
		"intercept_gps",
		registry.Component{
			Constructor: func(
				ctx context.Context,
				r robot.Robot,
				config config.Component,
				logger golog.Logger,
			) (interface{}, error) {
				return newInterceptingGPSBase(r, config)
			},
		},
	)
}

// GPS is a fake gps device that always returns the set location.
type GPS struct {
	mu         sync.Mutex
	Name       string
	Latitude   float64
	Longitude  float64
	altitude   float64
	speed      float64
	activeSats int
	totalSats  int
	hAcc       float64
	vAcc       float64
	valid      bool
}

// Readings always returns the set values.
func (g *GPS) Readings(ctx context.Context) ([]interface{}, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	return []interface{}{g.Latitude, g.Longitude}, nil
}

// Location always returns the set values.
func (g *GPS) Location(ctx context.Context) (*geo.Point, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	return geo.NewPoint(g.Latitude, g.Longitude), nil
}

// Altitude returns the set value.
func (g *GPS) Altitude(ctx context.Context) (float64, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.altitude, nil
}

// Speed returns the set value.
func (g *GPS) Speed(ctx context.Context) (float64, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.speed, nil
}

// ReadSatellites returns the set values.
func (g *GPS) ReadSatellites(ctx context.Context) (int, int, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.activeSats, g.totalSats, nil
}

// ReadAccuracy returns the set values.
func (g *GPS) ReadAccuracy(ctx context.Context) (float64, float64, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.hAcc, g.vAcc, nil
}

// ReadValid returns the set value.
func (g *GPS) ReadValid(ctx context.Context) (bool, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.valid, nil
}

// RunCommand runs an arbitrary command.
func (g *GPS) RunCommand(ctx context.Context, name string, args map[string]interface{}) (map[string]interface{}, error) {
	switch name {
	case "set_location":
		g.mu.Lock()
		defer g.mu.Unlock()
		lat, ok := args["latitude"].(float64)
		if !ok || args["latitude"] == nil {
			return nil, errors.New("expected latitude")
		}
		lng, ok := args["longitude"].(float64)
		if !ok || args["longitude"] == nil {
			return nil, errors.New("expected longitude")
		}
		g.Latitude = lat
		g.Longitude = lng
	default:
		return nil, errors.Errorf("unknown command %q", name)
	}
	return map[string]interface{}(nil), nil
}

type interceptingGPSBase struct {
	b       base.Base
	g       *GPS
	bearing float64 // [0-360)
}

func newInterceptingGPSBase(r robot.Robot, c config.Component) (*interceptingGPSBase, error) {
	baseName := c.Attributes.String("base")
	if baseName == "" {
		return nil, errors.New("'base' name must be set")
	}
	gpsName := c.Attributes.String("gps")
	if gpsName == "" {
		return nil, errors.New("'gps' name must be set")
	}
	b, ok := r.BaseByName(baseName)
	if !ok {
		return nil, errors.Errorf("no base named %q", baseName)
	}
	s, ok := r.ResourceByName(gps.Named(gpsName))
	if !ok {
		return nil, errors.Errorf("no gps named %q", gpsName)
	}
	gpsDevice, ok := s.(gps.GPS)
	if !ok {
		return nil, errors.Errorf("%q is not a GPS device", gpsName)
	}
	fakeG, ok := utils.UnwrapProxy(gpsDevice).(*GPS)
	if !ok {
		return nil, utils.NewUnexpectedTypeError(fakeG, utils.UnwrapProxy(gpsDevice))
	}

	lat := c.Attributes.Float64("start_latitude", 0)
	lng := c.Attributes.Float64("start_longitude", 0)

	fakeG.Latitude = lat
	fakeG.Longitude = lng
	return &interceptingGPSBase{b: b, g: fakeG}, nil
}

func (b *interceptingGPSBase) MoveStraight(ctx context.Context, distanceMillis int, millisPerSec float64, block bool) error {
	loc, err := b.g.Location(ctx)
	if err != nil {
		return err
	}
	err = b.b.MoveStraight(ctx, distanceMillis, millisPerSec, true)
	if err != nil {
		return err
	}
	distKilos := float64(distanceMillis) / 1000 / 1000
	newLoc := loc.PointAtDistanceAndBearing(distKilos, b.bearing)
	// set new location to be where we "perfectly" move to based on bearing
	b.g.Latitude = newLoc.Lat()
	b.g.Longitude = newLoc.Lng()
	return nil
}

// MoveArc allows the motion along an arc defined by speed, distance and angular velocity (TBD).
func (b *interceptingGPSBase) MoveArc(ctx context.Context, distanceMillis int, millisPerSec float64, angleDeg float64, block bool) error {
	return nil
}

func (b *interceptingGPSBase) Spin(ctx context.Context, angleDeg float64, degsPerSec float64, block bool) error {
	err := b.b.Spin(ctx, angleDeg, degsPerSec, true)
	if err != nil {
		return err
	}
	b.bearing = utils.ModAngDeg(b.bearing + angleDeg)
	return nil
}

func (b *interceptingGPSBase) WidthGet(ctx context.Context) (int, error) {
	return 600, nil
}

func (b *interceptingGPSBase) Stop(ctx context.Context) error {
	return nil
}
