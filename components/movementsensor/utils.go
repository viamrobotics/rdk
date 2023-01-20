package movementsensor

import (
	"errors"
	"math"

	geo "github.com/kellydunn/golang-geo"

	"go.viam.com/rdk/utils"
)

// GetHeading calculates bearing and absolute heading angles given 2 MovementSensor coordinates
// 0 degrees indicate North, 90 degrees indicate East and so on.
func GetHeading(gps1, gps2 *geo.Point, yawOffset float64) (float64, float64, float64) {
	// convert latitude and longitude readings from degrees to radians
	gps1Lat := utils.DegToRad(gps1.Lat())
	gps1Long := utils.DegToRad(gps1.Lng())
	gps2Lat := utils.DegToRad(gps2.Lat())
	gps2Long := utils.DegToRad(gps2.Lng())

	// calculate bearing from gps1 to gps 2
	dLon := gps2Long - gps1Long
	y := math.Sin(dLon) * math.Cos(gps2Lat)
	x := math.Cos(gps1Lat)*math.Sin(gps2Lat) - math.Sin(gps1Lat)*math.Cos(gps2Lat)*math.Cos(dLon)
	brng := utils.RadToDeg(math.Atan2(y, x))

	// maps bearing to 0-360 degrees
	if brng < 0 {
		brng += 360
	}

	// calculate absolute heading from bearing, accounting for yaw offset
	// e.g if the MovementSensor antennas are mounted on the left and right sides of the robot,
	// the yaw offset would be roughly 90 degrees
	var standardBearing float64
	if brng > 180 {
		standardBearing = -(360 - brng)
	} else {
		standardBearing = brng
	}
	heading := brng - yawOffset

	// make heading positive again
	if heading < 0 {
		diff := math.Abs(heading)
		heading = 360 - diff
	}

	return brng, heading, standardBearing
}

var (
	// ErrMethodUnimplementedAccuracy returns error if the Accuracy method is unimplemented.
	ErrMethodUnimplementedAccuracy = errors.New("Accuracy Unimplemented")
	// ErrMethodUnimplementedPosition returns error if the Position method is unimplemented.
	ErrMethodUnimplementedPosition = errors.New("Position Unimplemented")
	// ErrMethodUnimplementedOrientation returns error if the Orientation method is unimplemented.
	ErrMethodUnimplementedOrientation = errors.New("Orientation Unimplemented")
	// ErrMethodUnimplementedLinearVelocity returns error if the LinearVelocity method is unimplemented.
	ErrMethodUnimplementedLinearVelocity = errors.New("LinearVelocity Unimplemented")
	// ErrMethodUnimplementedAngularVelocity returns error if the AngularVelocity method is unimplemented.
	ErrMethodUnimplementedAngularVelocity = errors.New("AngularVelocity Unimplemented")
	// ErrMethodUnimplementedCompassHeading returns error if the CompassHeading method is unimplemented.
	ErrMethodUnimplementedCompassHeading = errors.New("CompassHeading Unimplemented")
	// ErrMethodUnimplementedReadings returns error if the Readings method is unimplemented.
	ErrMethodUnimplementedReadings = errors.New("Readings Unimplemented")
	// ErrMethodUnimplementedProperties returns error if the Properties method is unimplemented.
	ErrMethodUnimplementedProperties = errors.New("Properties Unimplemented")
	// ErrMethodUnimplementedLinearAcceleration returns error if Linear Acceleration is unimplemented.
	ErrMethodUnimplementedLinearAcceleration = errors.New("linear acceleration unimplemented")
)
