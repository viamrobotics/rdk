// Package dualgps implements a movement sensor that calculates compass heading
// from two gps movement sensors
package dualgps

import (
	"context"
	"errors"
	"math"
	"sync"

	"github.com/golang/geo/r3"
	geo "github.com/kellydunn/golang-geo"

	"go.viam.com/rdk/components/movementsensor"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/utils"
)

// the default offset between the two gps devices describes a setup where
// one gps is mounted on the right side of the base and
// the other gps is mounted on the left side of the base.
// This driver is not guaranteed to be performant with
// non-rtk corrected gps modules with larger error in their position.
//   ___________
//  |   base    |
//  |           |
// GPS2       GPS1
//  |           |
//  |___________|

const defaultOffsetDegrees = 90.0

var model = resource.DefaultModelFamily.WithModel("dual-gps-rtk")

// Config is used for converting the movementsensor attributes.
type Config struct {
	Gps1   string   `json:"first_gps"`
	Gps2   string   `json:"second_gps"`
	Offset *float64 `json:"offset_degrees,omitempty"`
}

// Validate validates the dual gps model's config to
// make sure that it has two gps movement sensors.
func (c *Config) Validate(path string) ([]string, error) {
	var deps []string

	if c.Gps1 == "" {
		return nil, resource.NewConfigValidationFieldRequiredError(path, "first_gps")
	}
	deps = append(deps, c.Gps1)

	if c.Gps2 == "" {
		return nil, resource.NewConfigValidationFieldRequiredError(path, "second_gps")
	}
	deps = append(deps, c.Gps2)

	if c.Offset != nil && (*c.Offset < 0 || *c.Offset > 360) {
		return nil, resource.NewConfigValidationError(
			path,
			errors.New("this driver only allows offset values from 0 to 360"))
	}
	return deps, nil
}

func init() {
	resource.RegisterComponent(
		movementsensor.API,
		model,
		resource.Registration[movementsensor.MovementSensor, *Config]{Constructor: newDualGPS})
}

type dualGPS struct {
	resource.Named
	logger logging.Logger
	mu     sync.Mutex
	offset float64

	gps1 movementsensor.MovementSensor
	gps2 movementsensor.MovementSensor
}

// newDualGPS makes a new movement sensor.
func newDualGPS(ctx context.Context, deps resource.Dependencies, conf resource.Config, logger logging.Logger,
) (movementsensor.MovementSensor, error) {
	dg := dualGPS{
		Named:  conf.ResourceName().AsNamed(),
		logger: logger,
	}

	if err := dg.Reconfigure(ctx, deps, conf); err != nil {
		return nil, err
	}
	return &dg, nil
}

func (dg *dualGPS) Reconfigure(ctx context.Context, deps resource.Dependencies, conf resource.Config) error {
	newConf, err := resource.NativeConfig[*Config](conf)
	if err != nil {
		return err
	}

	dg.mu.Lock()
	defer dg.mu.Unlock()

	first, err := movementsensor.FromDependencies(deps, newConf.Gps1)
	if err != nil {
		return err
	}
	dg.gps1 = first

	second, err := movementsensor.FromDependencies(deps, newConf.Gps2)
	if err != nil {
		return err
	}
	dg.gps2 = second

	dg.offset = defaultOffsetDegrees
	if newConf.Offset != nil {
		dg.offset = *newConf.Offset
	}

	dg.logger.Debug(
		"using gps named %v as first gps and gps named %v as second gps, with an offset of %v",
		first.Name().ShortName(),
		second.Name().ShortName(),
		dg.offset,
	)

	return nil
}

// getHeading calculates bearing, absolute heading and standardBearing angles given 2 geoPoint coordinates
// heading: 0 degrees is North, 90 degrees is East, 180 degrees is South, 270 is West.
// bearing: 0 degrees is North, 90 degrees is East, 180 degrees is South, 270 is West.
// standarBearing: 0 degrees is North, 90 degrees is East, 180 degrees is South, -90 is West.
// reference: https://www.igismap.com/formula-to-find-bearing-or-heading-angle-between-two-points-latitude-longitude/
func getHeading(firstPoint, secondPoint *geo.Point, yawOffset float64,
) (float64, float64, float64) {
	// convert latitude and longitude readings from degrees to radians
	// so we can use go's periodic math functions.
	firstLat := utils.DegToRad(firstPoint.Lat())
	firstLong := utils.DegToRad(firstPoint.Lng())
	secondLat := utils.DegToRad(secondPoint.Lat())
	secondLong := utils.DegToRad(secondPoint.Lng())

	// calculate the bearing between gps1 and gps2.
	deltaLong := secondLong - firstLong
	y := math.Sin(deltaLong) * math.Cos(secondLat)
	x := math.Cos(firstLat)*math.Sin(secondLat) - math.Sin(firstLat)*math.Cos(secondLat)*math.Cos(deltaLong)
	// return the standard bearing in the -180 to 180 range, with North being 0, East being 90
	// South being 180/-180 and West being -90
	standardBearing := utils.RadToDeg(math.Atan2(y, x))

	// maps the bearing to the range of 0-360 degrees.
	bearing := standardBearing
	if bearing < 0 {
		bearing += 360
	}

	// calculate heading from bearing, accounting for yaw offset between the two gps
	// e.g if the MovementSensor antennas are mounted on the left and right sides of the robot,
	// the yaw offset would be roughly 90 degrees
	heading := bearing - yawOffset
	// make heading positive again
	if heading < 0 {
		heading += 360
	}

	return bearing, heading, standardBearing
}

func (dg *dualGPS) CompassHeading(ctx context.Context, extra map[string]interface{}) (float64, error) {
	dg.mu.Lock()
	defer dg.mu.Unlock()

	geoPoint1, _, err := dg.gps1.Position(context.Background(), extra)
	if err != nil {
		return math.NaN(), err
	}

	geoPoint2, _, err := dg.gps2.Position(context.Background(), extra)
	if err != nil {
		return math.NaN(), err
	}

	_, heading, _ := getHeading(geoPoint1, geoPoint2, dg.offset)
	return heading, nil
}

func (dg *dualGPS) Properties(ctx context.Context, extra map[string]interface{}) (*movementsensor.Properties, error) {
	return &movementsensor.Properties{
		CompassHeadingSupported:     true,
		PositionSupported:           false,
		LinearVelocitySupported:     false,
		AngularVelocitySupported:    false,
		OrientationSupported:        false,
		LinearAccelerationSupported: false,
	}, nil
}

func (dg *dualGPS) Readings(ctx context.Context, extra map[string]interface{}) (map[string]interface{}, error) {
	return movementsensor.DefaultAPIReadings(ctx, dg, extra)
}

// Unimplemented functions.
func (dg *dualGPS) Position(ctx context.Context, extra map[string]interface{}) (*geo.Point, float64, error) {
	return geo.NewPoint(math.NaN(), math.NaN()), math.NaN(), movementsensor.ErrMethodUnimplementedPosition
}

func (dg *dualGPS) LinearAcceleration(ctx context.Context, extra map[string]interface{}) (r3.Vector, error) {
	return r3.Vector{}, movementsensor.ErrMethodUnimplementedLinearAcceleration
}

func (dg *dualGPS) LinearVelocity(ctx context.Context, extra map[string]interface{}) (r3.Vector, error) {
	return r3.Vector{}, movementsensor.ErrMethodUnimplementedLinearVelocity
}

func (dg *dualGPS) AngularVelocity(ctx context.Context, extra map[string]interface{}) (spatialmath.AngularVelocity, error) {
	return spatialmath.AngularVelocity{}, movementsensor.ErrMethodUnimplementedAngularVelocity
}

func (dg *dualGPS) Orientation(ctx context.Context, extra map[string]interface{}) (spatialmath.Orientation, error) {
	return spatialmath.NewZeroOrientation(), movementsensor.ErrMethodUnimplementedOrientation
}

func (dg *dualGPS) Accuracy(ctx context.Context, extra map[string]interface{}) (map[string]float32,
	float32, float32, movementsensor.NmeaGGAFixType, float32, error) {
	return map[string]float32{}, 0, 0, 0, 0, movementsensor.ErrMethodUnimplementedAccuracy
}

func (dg *dualGPS) DoCommand(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	return map[string]interface{}{}, resource.ErrDoUnimplemented
}

func (dg *dualGPS) Close(ctx context.Context) error {
	return nil
}
