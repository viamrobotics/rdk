package movementsensor

import (
	"errors"
	"math"
	"sync"

	geo "github.com/kellydunn/golang-geo"
)

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
	return LastError{errs: make([]error, size), threshold: threshold, size: size}
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

// LastPosition stores the last position seen by the movement sensor.
type LastPosition struct {
	lastposition *geo.Point
	mu           sync.Mutex
}

// NewLastPosition creates a new point that's (NaN, NaN)
// go-staticcheck.
func NewLastPosition() LastPosition {
	return LastPosition{lastposition: geo.NewPoint(math.NaN(), math.NaN())}
}

// GetLastPosition returns the last known position.
func (lp *LastPosition) GetLastPosition() *geo.Point {
	lp.mu.Lock()
	defer lp.mu.Unlock()
	return lp.lastposition
}

// SetLastPosition updates the last known position.
func (lp *LastPosition) SetLastPosition(position *geo.Point) {
	lp.mu.Lock()
	defer lp.mu.Unlock()
	lp.lastposition = position
}

// ArePointsEqual checks if two geo.Point instances are equal.
func (lp *LastPosition) ArePointsEqual(p1, p2 *geo.Point) bool {
	if p1 == nil || p2 == nil {
		return p1 == p2
	}
	return p1.Lng() == p2.Lng() && p1.Lat() == p2.Lat()
}

// IsZeroPosition checks if a geo.Point represents the zero position (0, 0).
func (lp *LastPosition) IsZeroPosition(p *geo.Point) bool {
	return p.Lng() == 0 && p.Lat() == 0
}

// IsPositionNaN checks if a geo.Point in math.NaN().
func (lp *LastPosition) IsPositionNaN(p *geo.Point) bool {
	return math.IsNaN(p.Lng()) && math.IsNaN(p.Lat())
}

// LastCompassHeading store the last valid compass heading seen by the movement sensor.
// This is really just an atomic float64, analogous to the atomic ints provided in the
// "sync/atomic" package.
type LastCompassHeading struct {
	lastcompassheading float64
	mu                 sync.Mutex
}

// NewLastCompassHeading create a new LastCompassHeading.
func NewLastCompassHeading() LastCompassHeading {
	return LastCompassHeading{lastcompassheading: math.NaN()}
}

// GetLastCompassHeading returns the last compass heading stored.
func (lch *LastCompassHeading) GetLastCompassHeading() float64 {
	lch.mu.Lock()
	defer lch.mu.Unlock()
	return lch.lastcompassheading
}

// SetLastCompassHeading sets lastcompassheading to the value given in the function.
func (lch *LastCompassHeading) SetLastCompassHeading(compassheading float64) {
	lch.mu.Lock()
	defer lch.mu.Unlock()
	lch.lastcompassheading = compassheading
}

// PMTKAddChk adds PMTK checksums to commands by XORing the bytes together.
func PMTKAddChk(data []byte) []byte {
	chk := PMTKChecksum(data)
	newCmd := []byte("$")
	newCmd = append(newCmd, data...)
	newCmd = append(newCmd, []byte("*")...)
	newCmd = append(newCmd, chk)
	return newCmd
}

// PMTKChecksum calculates the checksum of a byte array by performing an XOR operation on each byte.
func PMTKChecksum(data []byte) byte {
	var chk byte
	for _, b := range data {
		chk ^= b
	}
	return chk
}
