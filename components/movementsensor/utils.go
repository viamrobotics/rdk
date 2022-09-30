package movementsensor

import (
	"errors"
	"math"

	geo "github.com/kellydunn/golang-geo"
	errs "github.com/pkg/errors"

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

type mStr string

const (
	posStr      = mStr("Position")
	oriStr      = mStr("Orientation")
	angvelStr   = mStr("AngularVelocity")
	linvelStr   = mStr("LinearVelocity")
	compStr     = mStr("CompassHeading")
	accuracyStr = mStr("Accuracy")
	readStr     = mStr("Readings")
)

// ErrMethodUnimplemented implements unused method errors.
func ErrMethodUnimplemented(method mStr) error {
	switch method {
	case posStr:
		return errors.New("Position Unimplemented")
	case oriStr:
		return errors.New("Orientation Unimplemented")
	case angvelStr:
		return errors.New("LinearVelocity Unimplemented")
	case linvelStr:
		return errors.New("AngularVelocity Unimplemented")
	case compStr:
		return errors.New("CompassHeading Unimplemented")
	case accuracyStr:
		return errors.New("Accuracy Unimplemented")
	case readStr:
		return errors.New("Readings Unimplemented")
	default:
		return errs.Errorf("unknown method name %s used for creating unimerror", method)
	}
}
