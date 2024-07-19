// Package gpsrtk implements a GPS RTK that we communicate with via either serial port or I2C.
package gpsrtk

/*
	This package supports GPS RTK (Real Time Kinematics), which takes in the normal signals
	from the GNSS (Global Navigation Satellite Systems) along with a correction stream to achieve
	positional accuracy (accuracy tbd). This file is the main implementation, agnostic of how we
	communicate with the chip. This package has ways to communicate with the chip via the serial
	port and the I2C bus.

	Example GPS RTK chip datasheet:
	https://content.u-blox.com/sites/default/files/ZED-F9P-04B_DataSheet_UBX-21044850.pdf

	Ntrip Documentation:
	https://gssc.esa.int/wp-content/uploads/2018/07/NtripDocumentation.pdf

*/

import (
	"context"
	"errors"
	"io"
	"math"
	"sync"

	"github.com/go-gnss/rtcm/rtcm3"
	"github.com/golang/geo/r3"
	geo "github.com/kellydunn/golang-geo"

	"go.viam.com/rdk/components/movementsensor"
	"go.viam.com/rdk/components/movementsensor/gpsutils"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/utils"
)

// gpsrtk is an nmea movementsensor model that can intake RTK correction data.
type gpsrtk struct {
	resource.Named
	resource.AlwaysRebuild
	logger logging.Logger

	workers utils.StoppableWorkers

	err      movementsensor.LastError
	isClosed bool

	mu sync.Mutex

	// everything below this comment is protected by mu
	ntripClient      *gpsutils.NtripInfo
	cachedData       *gpsutils.CachedData
	correctionWriter io.WriteCloser
	writePath        string
	wbaud            int
	isVirtualBase    bool
	vrs              *gpsutils.VRS
}

func (g *gpsrtk) start() error {
	if g.workers != nil {
		return errors.New("do not double-start background goroutines")
	}
	g.workers = utils.NewStoppableWorkers(g.receiveAndWriteCorrectionData)
	return nil
}

// closeCorrectionWriter closes the correctionWriter.
func (g *gpsrtk) closeCorrectionWriter() {
	g.mu.Lock()
	defer g.mu.Unlock()

	if g.correctionWriter != nil {
		err := g.correctionWriter.Close()
		if err != nil {
			g.logger.Errorf("Error closing port: %v", err)
		}
	}
}

// connectAndParseSourceTable connects to the NTRIP caster, gets and parses source table
// from the caster.
func (g *gpsrtk) connectAndParseSourceTable(cancelCtx context.Context) error {
	if err := cancelCtx.Err(); err != nil {
		return g.err.Get()
	}

	err := g.ntripClient.Connect(cancelCtx, g.logger)
	if err != nil {
		g.err.Set(err)
		return g.err.Get()
	}

	g.logger.Debug("getting source table")

	srcTable, err := g.ntripClient.ParseSourcetable(g.logger)
	if err != nil {
		g.logger.Errorf("failed to get source table: %v", err)
		return err
	}
	g.logger.Debugf("sourceTable is: %v\n", srcTable)

	g.logger.Debug("got sourcetable, parsing it...")
	g.isVirtualBase, err = gpsutils.HasVRSStream(srcTable, g.ntripClient.MountPoint)
	if err != nil {
		g.logger.Errorf("can't find mountpoint in source table, found err %v\n", err)
		return err
	}

	return nil
}

func (g *gpsrtk) getStream(cancelCtx context.Context) (*rtcm3.Scanner, error) {
	var streamSource io.Reader

	if g.isVirtualBase {
		g.logger.Debug("connecting to Virtual Reference Station")
		err := g.getNtripFromVRS(cancelCtx)
		if err != nil {
			return nil, err
		}
		streamSource = g.vrs.GetReaderWriter()
	} else {
		g.logger.Debug("connecting to NTRIP stream from static mount point...")
		var err error
		streamSource, err = g.ntripClient.GetStreamFromMountPoint(cancelCtx, g.logger)
		if err != nil {
			return nil, err
		}
	}
	reader := io.TeeReader(streamSource, g.correctionWriter)
	scanner := rtcm3.NewScanner(reader)
	return &scanner, nil
}

// receiveAndWriteCorrectionData connects to the NTRIP receiver and sends the correction stream to
// the MovementSensor.
func (g *gpsrtk) receiveAndWriteCorrectionData(cancelCtx context.Context) {
	defer g.closeCorrectionWriter()

	err := g.connectAndParseSourceTable(cancelCtx)
	if err != nil {
		g.logger.Errorf("unable to parse source table! Aborting: %w", err)
		return
	}

	// While we're supposed to keep running, (re)connect to the caster.
	for !g.isClosed && cancelCtx.Err() == nil {
		scanner, err := g.getStream(cancelCtx)
		if err != nil {
			g.logger.Errorf("unable to get NTRIP stream! Aborting: %w", err)
			return
		}

		for err == nil { // Keep checking our connection until it fails and needs to reconnect
			if g.isClosed || cancelCtx.Err() != nil {
				return
			}

			// Calling NextMessage() reads from the scanner until a valid message is found, and
			// returns that. We don't care about the message: we care that the scanner is able to
			// read messages at all! So, focus on whether the scanner had errors (which indicate we
			// need to reconnect to the mount point), and not the message itself.
			_, err = scanner.NextMessage()
		}
		g.logger.Debugf("no longer connected to NTRIP scanner: %s", err)
	}
}

// Most of the movementsensor functions here don't have mutex locks since g.cachedData is protected by
// it's own mutex and not having mutex around g.err is alright.

// Position returns the current geographic location of the MOVEMENTSENSOR.
func (g *gpsrtk) Position(ctx context.Context, extra map[string]interface{}) (*geo.Point, float64, error) {
	nanPoint := geo.NewPoint(math.NaN(), math.NaN())

	lastError := g.err.Get()
	if lastError != nil {
		return nanPoint, math.NaN(), lastError
	}

	position, alt, err := g.cachedData.Position(ctx, extra)
	if err != nil {
		return nanPoint, math.NaN(), err
	}

	if movementsensor.IsPositionNaN(position) {
		position = nanPoint
	}
	return position, alt, nil
}

// LinearVelocity passthrough.
func (g *gpsrtk) LinearVelocity(ctx context.Context, extra map[string]interface{}) (r3.Vector, error) {
	lastError := g.err.Get()
	if lastError != nil {
		return r3.Vector{}, lastError
	}

	return g.cachedData.LinearVelocity(ctx, extra)
}

// LinearAcceleration passthrough.
func (g *gpsrtk) LinearAcceleration(ctx context.Context, extra map[string]interface{}) (r3.Vector, error) {
	lastError := g.err.Get()
	if lastError != nil {
		return r3.Vector{}, lastError
	}
	return g.cachedData.LinearAcceleration(ctx, extra)
}

// AngularVelocity passthrough.
func (g *gpsrtk) AngularVelocity(ctx context.Context, extra map[string]interface{}) (spatialmath.AngularVelocity, error) {
	lastError := g.err.Get()
	if lastError != nil {
		return spatialmath.AngularVelocity{}, lastError
	}

	return g.cachedData.AngularVelocity(ctx, extra)
}

// CompassHeading passthrough.
func (g *gpsrtk) CompassHeading(ctx context.Context, extra map[string]interface{}) (float64, error) {
	lastError := g.err.Get()
	if lastError != nil {
		return 0, lastError
	}
	return g.cachedData.CompassHeading(ctx, extra)
}

// Orientation passthrough.
func (g *gpsrtk) Orientation(ctx context.Context, extra map[string]interface{}) (spatialmath.Orientation, error) {
	lastError := g.err.Get()
	if lastError != nil {
		return spatialmath.NewZeroOrientation(), lastError
	}
	return g.cachedData.Orientation(ctx, extra)
}

// Properties passthrough.
func (g *gpsrtk) Properties(ctx context.Context, extra map[string]interface{}) (*movementsensor.Properties, error) {
	lastError := g.err.Get()
	if lastError != nil {
		return &movementsensor.Properties{}, lastError
	}

	return g.cachedData.Properties(ctx, extra)
}

// Accuracy passthrough.
func (g *gpsrtk) Accuracy(ctx context.Context, extra map[string]interface{}) (*movementsensor.Accuracy, error,
) {
	lastError := g.err.Get()
	if lastError != nil {
		return nil, lastError
	}

	return g.cachedData.Accuracy(ctx, extra)
}

// Readings will use the default MovementSensor Readings if not provided.
func (g *gpsrtk) Readings(ctx context.Context, extra map[string]interface{}) (map[string]interface{}, error) {
	readings, err := movementsensor.DefaultAPIReadings(ctx, g, extra)
	if err != nil {
		return nil, err
	}

	commonReadings := g.cachedData.GetCommonReadings(ctx)

	readings["fix"] = commonReadings.FixValue
	readings["satellites_in_view"] = commonReadings.SatsInView
	readings["satellites_in_use"] = commonReadings.SatsInUse

	return readings, nil
}

// Close shuts down the gpsrtk.
func (g *gpsrtk) Close(ctx context.Context) error {
	g.logger.Debug("Closing GPS RTK")
	g.workers.Stop()

	g.mu.Lock()
	defer g.mu.Unlock()

	if err := g.cachedData.Close(ctx); err != nil {
		return err
	}

	if g.correctionWriter != nil {
		if err := g.correctionWriter.Close(); err != nil {
			g.isClosed = true
			return err
		}
		g.correctionWriter = nil
	}

	if g.vrs != nil {
		if err := g.vrs.Close(ctx); err != nil {
			return err
		}
	}

	if err := g.ntripClient.Close(ctx); err != nil {
		return err
	}

	if err := g.err.Get(); err != nil && !errors.Is(err, context.Canceled) {
		return err
	}

	g.logger.Debug("GPS RTK is closed")
	return nil
}

// getNtripFromVRS sends GGA messages to the NTRIP Caster over a TCP connection
// to get the NTRIP steam when the mount point is a Virtual Reference Station.
func (g *gpsrtk) getNtripFromVRS(cancelCtx context.Context) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	var err error
	if g.vrs != nil {
		if err := g.vrs.Close(cancelCtx); err != nil {
			return err
		}
		g.vrs = nil
	}
	g.vrs, err = gpsutils.ConnectToVirtualBase(cancelCtx, g.ntripClient, g.cachedData.GGA, g.logger)
	if err != nil {
		return err
	}

	return nil
}
