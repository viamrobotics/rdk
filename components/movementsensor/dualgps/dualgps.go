package dualgps

import (
	"context"
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
// one gps is mounted on the right side of the base and one gps is mounted on the left side of the base
//   ___________
//  |	base	|
//  |			|
// GPS2			GPS1
//  |			|
//  |___________|

const defaultOffsetDegrees = 90.0

var model = resource.DefaultModelFamily.WithModel("dual-gps-rtk")

// Config is used for converting fake movementsensor attributes.
type Config struct {
	Gps1   string  `json:"first_gps"`
	Gps2   string  `json:"second_gps"`
	Offset float64 `json:"offset_degrees,omitempty"`
}

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

	gps1 gpsWithLock
	gps2 gpsWithLock
}

// newDualGPS makes a new fake movement sensor.
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
	if newConf.Gps1 != first.Name().ShortName() {
		dg.gps1.gps = first
	}

	second, err := movementsensor.FromDependencies(deps, newConf.Gps2)
	if err != nil {
		return err
	}
	if newConf.Gps2 != second.Name().ShortName() {
		dg.gps2.gps = second
	}

	dg.offset = defaultOffsetDegrees
	if newConf.Offset != 0 {
		dg.offset = newConf.Offset
	}

	return nil
}

type gpsWithLock struct {
	gps     movementsensor.MovementSensor
	gpsLock sync.Mutex
}

func (lgps *gpsWithLock) getPosition(ctx context.Context, extra map[string]interface{}) (*geo.Point, float64, error) {
	lgps.gpsLock.Lock()
	defer lgps.gpsLock.Unlock()
	return lgps.gps.Position(ctx, extra)
}

// Position gets the position of a fake movementsensor.
func (dg *dualGPS) Position(ctx context.Context, extra map[string]interface{}) (*geo.Point, float64, error) {
	return geo.NewPoint(math.NaN(), math.NaN()), math.NaN(), movementsensor.ErrMethodUnimplementedPosition
}

// getHeading calculates bearing and absolute heading angles given 2 geoPoint coordinates
// 0 degrees is North, 90 degrees is East, 180 degrees is South, 270 is West.
func getHeading(first, second *geo.Point, yawOffset float64) (float64, float64, float64) {
	// convert latitude and longitude readings from degrees to radians
	firstLat := utils.DegToRad(first.Lat())
	firstLong := utils.DegToRad(first.Lng())
	secondLat := utils.DegToRad(second.Lat())
	secondLong := utils.DegToRad(second.Lng())

	// calculate bearing from gps1 to gps 2
	deltaLong := secondLong - firstLong
	y := math.Sin(deltaLong) * math.Cos(secondLat)
	x := math.Cos(firstLat)*math.Sin(secondLat) - math.Sin(firstLat)*math.Cos(secondLat)*math.Cos(deltaLong)
	bearing := utils.RadToDeg(math.Atan2(y, x))

	// maps bearing to 0-360 degrees
	if bearing < 0 {
		bearing += 360
	}

	// calculate absolute heading from bearing, accounting for yaw offset
	// e.g if the MovementSensor antennas are mounted on the left and right sides of the robot,
	// the yaw offset would be roughly 90 degrees
	var standardBearing float64
	if bearing > 180 {
		standardBearing = -(360 - bearing)
	} else {
		standardBearing = bearing
	}
	heading := bearing - yawOffset

	// make heading positive again
	if heading < 0 {
		diff := math.Abs(heading)
		heading = 360 - diff
	}

	return bearing, heading, standardBearing
}

// CompassHeading gets the compass headings of a fake movementsensor.
func (dg *dualGPS) CompassHeading(ctx context.Context, extra map[string]interface{}) (float64, error) {
	geoPoint1, _, err := dg.gps1.getPosition(context.Background(), extra)
	if err != nil {
		return math.NaN(), err
	}

	geoPoint2, _, err := dg.gps2.getPosition(context.Background(), extra)
	if err != nil {
		return math.NaN(), err
	}

	bearing, _, _ := getHeading(geoPoint1, geoPoint2, 0)
	return bearing, nil
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

// Unimplemented functions
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

func (dg *dualGPS) Accuracy(ctx context.Context, extra map[string]interface{}) (map[string]float32, error) {
	return map[string]float32{}, movementsensor.ErrMethodUnimplementedAccuracy
}

func (dg *dualGPS) DoCommand(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	return map[string]interface{}{}, resource.ErrDoUnimplemented
}

func (dg *dualGPS) Close(ctx context.Context) error {
	return nil
}
