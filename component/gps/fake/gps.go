// Package fake implements a fake GPS.
package fake

import (
	"context"
	"sync"
	"time"

	"github.com/edaniels/golog"
	"github.com/golang/geo/r3"
	geo "github.com/kellydunn/golang-geo"
	"github.com/pkg/errors"
	goutils "go.viam.com/utils"

	"go.viam.com/rdk/component/base"
	"go.viam.com/rdk/component/generic"
	"go.viam.com/rdk/component/gps"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/utils"
)

func init() {
	registry.RegisterComponent(
		gps.Subtype,
		"fake",
		registry.Component{Constructor: func(
			ctx context.Context,
			deps registry.Dependencies,
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
				deps registry.Dependencies,
				config config.Component,
				logger golog.Logger,
			) (interface{}, error) {
				return newInterceptingGPSBase(deps, config)
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
	generic.Unimplemented
	mu                        sync.Mutex
	b                         base.Base
	g                         *GPS
	bearing                   float64 // [0-360)
	linearPower, angularPower r3.Vector

	cancelFunc              func()
	activeBackgroundWorkers sync.WaitGroup
}

func newInterceptingGPSBase(deps registry.Dependencies, c config.Component) (base.LocalBase, error) {
	baseName := c.Attributes.String("base")
	if baseName == "" {
		return nil, errors.New("'base' name must be set")
	}
	gpsName := c.Attributes.String("gps")
	if gpsName == "" {
		return nil, errors.New("'gps' name must be set")
	}
	b, err := base.FromDependencies(deps, baseName)
	if err != nil {
		return nil, err
	}
	gpsDevice, err := gps.FromDependencies(deps, gpsName)
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

	cancelCtx, cancelFunc := context.WithCancel(context.Background())

	intBase := &interceptingGPSBase{b: b, g: fakeG, cancelFunc: cancelFunc}
	intBase.activeBackgroundWorkers.Add(1)
	goutils.PanicCapturingGo(func() {
		defer intBase.activeBackgroundWorkers.Done()
		lastT := time.Now()
		for {
			if !goutils.SelectContextOrWait(cancelCtx, 200*time.Millisecond) {
				return
			}

			loc, err := intBase.g.ReadLocation(cancelCtx)
			if err != nil {
				continue
			}

			nextT := time.Now()
			delta := nextT.Sub(lastT)
			lastT = nextT

			intBase.mu.Lock()
			fixedAngular := r3.Vector{intBase.angularPower.Z, 0, 0}
			bearingVec := intBase.linearPower.Add(fixedAngular)
			intBase.mu.Unlock()
			power := bearingVec.Norm()
			angle1 := bearingVec.Angle(r3.Vector{1, 0, 0}).Degrees()
			angle2 := bearingVec.Angle(r3.Vector{0, 1, 0}).Degrees()
			angle := angle1 + 180

			if angle2 <= 90 {
				angle = utils.AntiCWDeg(angle)
			}

			distKilos := (maxMmPerSec * power * delta.Seconds()) / 1000 / 1000

			newLoc := loc.PointAtDistanceAndBearing(distKilos, angle)
			// set new location to be where we "perfectly" move to based on bearing
			intBase.g.mu.Lock()
			intBase.g.Latitude = newLoc.Lat()
			intBase.g.Longitude = newLoc.Lng()
			intBase.g.mu.Unlock()
		}
	})

	return intBase, nil
}

func (b *interceptingGPSBase) MoveStraight(ctx context.Context, distanceMm int, mmPerSec float64) error {
	loc, err := b.g.ReadLocation(ctx)
	if err != nil {
		return err
	}
	err = b.b.MoveStraight(ctx, distanceMm, mmPerSec)
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

func (b *interceptingGPSBase) Spin(ctx context.Context, angleDeg float64, degsPerSec float64) error {
	err := b.b.Spin(ctx, angleDeg, degsPerSec)
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

func (b *interceptingGPSBase) IsMoving(ctx context.Context) (bool, error) {
	return false, nil
}

const maxMmPerSec = 300

// SetPower sets power based on the linear and angular components. However, due to being fake and not
// wanting to support trajectory planning, we will use the Y component from linear and Z component
// from angular to represent X, Y unit vectors, respectively, from a joystick control. Using the result of
// the sum of these two vectors: the angle represents bearing and the magnitude represents power.
// You can think of this as using a joystick to control a base from a birds eye view on a map.
func (b *interceptingGPSBase) SetPower(ctx context.Context, linear, angular r3.Vector) error {
	b.mu.Lock()
	b.linearPower = linear
	b.angularPower = angular
	b.mu.Unlock()
	return nil
}

func (b *interceptingGPSBase) SetVelocity(ctx context.Context, linear, angular r3.Vector) error {
	return nil
}

func (b *interceptingGPSBase) Close() {
	b.cancelFunc()
	b.activeBackgroundWorkers.Wait()
}
