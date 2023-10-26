// Package rtkutils defines a gps and an rtk correction source
// which sends rtcm data to a child gps
// This is an Experimental package
package rtkutils

import (
	"context"
	"errors"
	"io"
	"math"
	"sync"

	"github.com/golang/geo/r3"
	geo "github.com/kellydunn/golang-geo"

	"go.viam.com/rdk/components/board"
	"go.viam.com/rdk/components/movementsensor"
	gpsnmea "go.viam.com/rdk/components/movementsensor/gpsnmea"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/spatialmath"
)

// A RTKMovementSensor is an NMEA MovementSensor model that can intake RTK correction data.
type RTKMovementSensor struct {
	resource.Named
	resource.AlwaysRebuild
	logger     logging.Logger
	cancelCtx  context.Context
	cancelFunc func()

	activeBackgroundWorkers sync.WaitGroup

	ntripMu     sync.Mutex
	ntripClient *NtripInfo
	ntripStatus bool

	err          movementsensor.LastError
	lastposition movementsensor.LastPosition

	Nmeamovementsensor gpsnmea.NmeaMovementSensor
	InputProtocol      string
	CorrectionWriter   io.ReadWriteCloser

	Bus       board.I2C
	Wbaud     int
	Addr      byte // for i2c only
	Writepath string
}

// GetStream attempts to connect to ntrip streak until successful connection or timeout.
func (g *RTKMovementSensor) GetStream(mountPoint string, maxAttempts int) error {
	success := false
	attempts := 0

	var rc io.ReadCloser
	var err error

	g.logger.Debug("Getting NTRIP stream")

	for !success && attempts < maxAttempts {
		select {
		case <-g.cancelCtx.Done():
			return errors.New("Canceled")
		default:
		}

		rc, err = func() (io.ReadCloser, error) {
			g.ntripMu.Lock()
			defer g.ntripMu.Unlock()
			return g.ntripClient.Client.GetStream(mountPoint)
		}()
		if err == nil {
			success = true
		}
		attempts++
	}

	if err != nil {
		g.logger.Errorf("Can't connect to NTRIP stream: %s", err)
		return err
	}

	g.logger.Debug("Connected to stream")
	g.ntripMu.Lock()
	defer g.ntripMu.Unlock()

	g.ntripClient.Stream = rc
	return g.err.Get()
}

// NtripStatus returns true if connection to NTRIP stream is OK, false if not.
func (g *RTKMovementSensor) NtripStatus() (bool, error) {
	g.ntripMu.Lock()
	defer g.ntripMu.Unlock()
	return g.ntripStatus, g.err.Get()
}

// Position returns the current geographic location of the MOVEMENTSENSOR.
func (g *RTKMovementSensor) Position(ctx context.Context, extra map[string]interface{}) (*geo.Point, float64, error) {
	g.ntripMu.Lock()
	lastError := g.err.Get()
	if lastError != nil {
		lastPosition := g.lastposition.GetLastPosition()
		g.ntripMu.Unlock()
		if lastPosition != nil {
			return lastPosition, 0, nil
		}
		return geo.NewPoint(math.NaN(), math.NaN()), math.NaN(), lastError
	}
	g.ntripMu.Unlock()

	position, alt, err := g.Nmeamovementsensor.Position(ctx, extra)
	if err != nil {
		// Use the last known valid position if current position is (0,0)/ NaN.
		if position != nil && (g.lastposition.IsZeroPosition(position) || g.lastposition.IsPositionNaN(position)) {
			lastPosition := g.lastposition.GetLastPosition()
			if lastPosition != nil {
				return lastPosition, alt, nil
			}
		}
		return geo.NewPoint(math.NaN(), math.NaN()), math.NaN(), err
	}

	// Check if the current position is different from the last position and non-zero
	lastPosition := g.lastposition.GetLastPosition()
	if !g.lastposition.ArePointsEqual(position, lastPosition) {
		g.lastposition.SetLastPosition(position)
	}

	// Update the last known valid position if the current position is non-zero
	if position != nil && !g.lastposition.IsZeroPosition(position) {
		g.lastposition.SetLastPosition(position)
	}

	return position, alt, nil
}

// LinearVelocity passthrough.
func (g *RTKMovementSensor) LinearVelocity(ctx context.Context, extra map[string]interface{}) (r3.Vector, error) {
	g.ntripMu.Lock()
	lastError := g.err.Get()
	if lastError != nil {
		defer g.ntripMu.Unlock()
		return r3.Vector{}, lastError
	}
	g.ntripMu.Unlock()

	return g.Nmeamovementsensor.LinearVelocity(ctx, extra)
}

// LinearAcceleration passthrough.
func (g *RTKMovementSensor) LinearAcceleration(ctx context.Context, extra map[string]interface{}) (r3.Vector, error) {
	lastError := g.err.Get()
	if lastError != nil {
		return r3.Vector{}, lastError
	}
	return g.Nmeamovementsensor.LinearAcceleration(ctx, extra)
}

// AngularVelocity passthrough.
func (g *RTKMovementSensor) AngularVelocity(ctx context.Context, extra map[string]interface{}) (spatialmath.AngularVelocity, error) {
	g.ntripMu.Lock()
	lastError := g.err.Get()
	if lastError != nil {
		defer g.ntripMu.Unlock()
		return spatialmath.AngularVelocity{}, lastError
	}
	g.ntripMu.Unlock()

	return g.Nmeamovementsensor.AngularVelocity(ctx, extra)
}

// CompassHeading passthrough.
func (g *RTKMovementSensor) CompassHeading(ctx context.Context, extra map[string]interface{}) (float64, error) {
	g.ntripMu.Lock()
	lastError := g.err.Get()
	if lastError != nil {
		defer g.ntripMu.Unlock()
		return 0, lastError
	}
	g.ntripMu.Unlock()

	return g.Nmeamovementsensor.CompassHeading(ctx, extra)
}

// Orientation passthrough.
func (g *RTKMovementSensor) Orientation(ctx context.Context, extra map[string]interface{}) (spatialmath.Orientation, error) {
	g.ntripMu.Lock()
	lastError := g.err.Get()
	if lastError != nil {
		defer g.ntripMu.Unlock()
		return spatialmath.NewZeroOrientation(), lastError
	}
	g.ntripMu.Unlock()

	return g.Nmeamovementsensor.Orientation(ctx, extra)
}

// ReadFix passthrough.
func (g *RTKMovementSensor) ReadFix(ctx context.Context) (int, error) {
	g.ntripMu.Lock()
	lastError := g.err.Get()
	if lastError != nil {
		defer g.ntripMu.Unlock()
		return 0, lastError
	}
	g.ntripMu.Unlock()

	return g.Nmeamovementsensor.ReadFix(ctx)
}

// Properties passthrough.
func (g *RTKMovementSensor) Properties(ctx context.Context, extra map[string]interface{}) (*movementsensor.Properties, error) {
	lastError := g.err.Get()
	if lastError != nil {
		return &movementsensor.Properties{}, lastError
	}

	return g.Nmeamovementsensor.Properties(ctx, extra)
}

// Accuracy passthrough.
func (g *RTKMovementSensor) Accuracy(ctx context.Context, extra map[string]interface{}) (map[string]float32, error) {
	lastError := g.err.Get()
	if lastError != nil {
		return map[string]float32{}, lastError
	}

	return g.Nmeamovementsensor.Accuracy(ctx, extra)
}

// Readings will use the default MovementSensor Readings if not provided.
func (g *RTKMovementSensor) Readings(ctx context.Context, extra map[string]interface{}) (map[string]interface{}, error) {
	readings, err := movementsensor.DefaultAPIReadings(ctx, g, extra)
	if err != nil {
		return nil, err
	}

	fix, err := g.ReadFix(ctx)
	if err != nil {
		return nil, err
	}

	readings["fix"] = fix

	return readings, nil
}

// Close shuts down the RTKMOVEMENTSENSOR.
func (g *RTKMovementSensor) Close(ctx context.Context) error {
	g.ntripMu.Lock()
	g.cancelFunc()

	if err := g.Nmeamovementsensor.Close(ctx); err != nil {
		g.ntripMu.Unlock()
		return err
	}

	// close ntrip writer
	if g.CorrectionWriter != nil {
		if err := g.CorrectionWriter.Close(); err != nil {
			g.ntripMu.Unlock()
			return err
		}
		g.CorrectionWriter = nil
	}

	// close ntrip client and stream
	if g.ntripClient.Client != nil {
		g.ntripClient.Client.CloseIdleConnections()
		g.ntripClient.Client = nil
	}

	if g.ntripClient.Stream != nil {
		if err := g.ntripClient.Stream.Close(); err != nil {
			g.ntripMu.Unlock()
			return err
		}
		g.ntripClient.Stream = nil
	}

	g.ntripMu.Unlock()
	g.activeBackgroundWorkers.Wait()

	if err := g.err.Get(); err != nil && !errors.Is(err, context.Canceled) {
		return err
	}
	return nil
}
