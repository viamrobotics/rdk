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

// ReadLocation always returns the set values.
func (g *GPS) ReadLocation(ctx context.Context) (*geo.Point, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	return geo.NewPoint(g.Latitude, g.Longitude), nil
}

// ReadAltitude returns the set value.
func (g *GPS) ReadAltitude(ctx context.Context) (float64, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.altitude, nil
}

// ReadSpeed returns the set value.
func (g *GPS) ReadSpeed(ctx context.Context) (float64, error) {
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

// Do runs an arbitrary command.
func (g *GPS) Do(ctx context.Context, args map[string]interface{}) (map[string]interface{}, error) {
	name, ok := args["command"]
	if !ok {
		return nil, errors.New("missing 'command' value")
	}
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
	b, err := base.FromRobot(r, baseName)
	if err != nil {
		return nil, err
	}
	gpsDevice, err := gps.FromRobot(r, gpsName)
	if err != nil {
		return nil, err
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

func (b *interceptingGPSBase) MoveStraight(ctx context.Context, distanceMm int, mmPerSec float64, block bool) error {
	loc, err := b.g.ReadLocation(ctx)
	if err != nil {
		return err
	}
	err = b.b.MoveStraight(ctx, distanceMm, mmPerSec, true)
	if err != nil {
		return err
	}
	distKilos := float64(distanceMm) / 1000 / 1000
	newLoc := loc.PointAtDistanceAndBearing(distKilos, b.bearing)
	// set new location to be where we "perfectly" move to based on bearing
	b.g.Latitude = newLoc.Lat()
	b.g.Longitude = newLoc.Lng()
	return nil
}

// MoveArc allows the motion along an arc defined by speed, distance and angular velocity (TBD).
func (b *interceptingGPSBase) MoveArc(ctx context.Context, distanceMm int, mmPerSec float64, angleDeg float64, block bool) error {
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

func (b *interceptingGPSBase) GetWidth(ctx context.Context) (int, error) {
	return 600, nil
}

func (b *interceptingGPSBase) Stop(ctx context.Context) error {
	return nil
}
