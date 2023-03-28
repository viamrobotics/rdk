package movementsensor

import (
	"errors"
	"math"
	"sync"

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

// LastError is an object that stores recent errors. If there have been sufficiently many recent
// errors, you can retrieve the most recent one.
type LastError struct {
	// These values are immutable
	size      int // The length of errs, below
	threshold int // How many items in errs must be non-nil for us to give back errors when asked

	// These values are mutable
	mu    sync.Mutex
	errs  []error // A list of recent errors, oldest to newest
	count int     // How many items in errs are non-nil
}

// NewLastError creates a LastError object which will let you retrieve the most recent error if at
// least `threshold` of the most recent `size` items put into it are non-nil.
func NewLastError(size, threshold int) LastError {
	return LastError{errs: make([]error, size), threshold: threshold}
}

// Set stores an error to be retrieved later.
func (le *LastError) Set(err error) {
	le.mu.Lock()
	defer le.mu.Unlock()

	// Remove the oldest error, and add the newest one.
	if le.errs[0] != nil {
		le.count--
	}
	if err != nil {
		le.count++
	}
	le.errs = append(le.errs[1:], err)
}

// Get returns the most-recently-stored non-nil error if we've had enough recent errors. If we're
// going to return a non-nil error, we also wipe out all other data so we don't return the same
// error again next time.
func (le *LastError) Get() error {
	le.mu.Lock()
	defer le.mu.Unlock()

	if le.count < le.threshold {
		// Keep our data, in case we're close to the threshold and will return an error next time.
		return nil
	}

	// Otherwise, find the most recent error, iterating through the list newest to oldest. Assuming
	// the threshold is at least 1, there is definitely some error in here to find.
	var errToReturn error
	for i := 0; i < len(le.errs); i++ {
		current := le.errs[len(le.errs)-1-i]
		if current == nil {
			continue
		}
		errToReturn = current
		break
	}

	// Wipe everything out
	le.errs = make([]error, le.size)
	le.count = 0
	return errToReturn
}
