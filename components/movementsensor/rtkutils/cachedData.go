package rtkutils

import (
	"context"
	"sync"

	geo "github.com/kellydunn/golang-geo"
	"github.com/pkg/errors"

	"go.viam.com/rdk/components/movementsensor"
)

var errNilLocation = errors.New("nil gps location, check nmea message parsing")

// CachedGpsData allows the use of any MovementSensor chip via a DataReader.
type CachedGpsData struct {
	mu           sync.RWMutex
	uncachedData GPSData

	err                movementsensor.LastError
	lastPosition       movementsensor.LastPosition
	lastCompassHeading movementsensor.LastCompassHeading
}

// NewCachedGpsData creates a new CachedGpsData object.
func NewCachedGpsData() CachedGpsData {
	return CachedGpsData{
		err:                movementsensor.NewLastError(1, 1),
		lastPosition:       movementsensor.NewLastPosition(),
		lastCompassHeading: movementsensor.NewLastCompassHeading(),
	}
}

// Position returns the position and altitide of the sensor, or an error.
func (g *CachedGpsData) Position(
	ctx context.Context, extra map[string]interface{},
) (*geo.Point, float64, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()

	lastPosition := g.lastPosition.GetLastPosition()
	currentPosition := g.uncachedData.Location

	if currentPosition == nil {
		return lastPosition, 0, errNilLocation
	}

	// if current position is (0,0) we will return the last non-zero position
	if movementsensor.IsZeroPosition(currentPosition) && !movementsensor.IsZeroPosition(lastPosition) {
		return lastPosition, g.uncachedData.Alt, g.err.Get()
	}

	// updating the last known valid position if the current position is non-zero
	if !movementsensor.IsZeroPosition(currentPosition) && !movementsensor.IsPositionNaN(currentPosition) {
		g.lastPosition.SetLastPosition(currentPosition)
	}

	return currentPosition, g.uncachedData.Alt, g.err.Get()
}
