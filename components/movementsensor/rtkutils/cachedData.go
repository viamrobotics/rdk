package rtkutils

import (
	"sync"

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

// NewCachedGpsData creates a new CachedGpsData object
func NewCachedGpsData() CachedGpsData {
	return CachedGpsData{
		err:                movementsensor.NewLastError(1, 1),
		lastPosition:       movementsensor.NewLastPosition(),
		lastCompassHeading: movementsensor.NewLastCompassHeading(),
	}
}
