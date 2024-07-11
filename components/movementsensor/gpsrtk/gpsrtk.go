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
	"fmt"
	"io"
	"math"
	"strings"
	"sync"

	"github.com/go-gnss/rtcm/rtcm3"
	"github.com/golang/geo/r3"
	geo "github.com/kellydunn/golang-geo"
	"go.viam.com/utils"

	"go.viam.com/rdk/components/movementsensor"
	"go.viam.com/rdk/components/movementsensor/gpsutils"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/spatialmath"
)

// gpsrtk is an nmea movementsensor model that can intake RTK correction data.
type gpsrtk struct {
	resource.Named
	resource.AlwaysRebuild
	logger     logging.Logger
	cancelCtx  context.Context
	cancelFunc func()

	activeBackgroundWorkers sync.WaitGroup

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
	// reader is the TeeReader to write the corrections stream to the gps chip.
	// Additionally used to scan RTCM messages to ensure there are no errors from the streams
	reader io.Reader
}

func (g *gpsrtk) start() error {
	err := g.connectToNTRIP()
	if err != nil {
		return err
	}
	g.activeBackgroundWorkers.Add(1)
	utils.PanicCapturingGo(g.receiveAndWriteCorrectionData)
	return g.err.Get()
}

// getStreamFromMountPoint attempts to connect to ntrip stream. We give up after maxAttempts unsuccessful tries.
func (g *gpsrtk) getStreamFromMountPoint(mountPoint string, maxAttempts int) error {
	success := false
	attempts := 0

	// setting the Timeout to 0 on the http client to prevent the ntrip stream from canceling itself.
	// ntrip.NewClient() defaults sets this value to 15 seconds, which causes us to disconnect
	// the ntrip stream and require a reconnection.
	// Setting the Timeout on the http client to be 0 removes the timeout. It's possible we want to have different
	// Additionally, this should be tested with other CORS.
	g.ntripClient.Client.Timeout = 0

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

	g.mu.Lock()
	defer g.mu.Unlock()

	g.ntripClient.Stream = rc
	return g.err.Get()
}

// closePort closes the correctionWriter.
func (g *gpsrtk) closePort() {
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
func (g *gpsrtk) connectAndParseSourceTable() error {
	if err := g.cancelCtx.Err(); err != nil {
		return g.err.Get()
	}

	err := g.ntripClient.Connect(g.cancelCtx, g.logger)
	if err != nil {
		g.err.Set(err)
		return g.err.Get()
	}

	if !g.ntripClient.Client.IsCasterAlive() {
		g.logger.Infof("caster %s seems to be down, retrying", g.ntripClient.URL)
		attempts := 0
		// we will try to connect to the caster five times if it's down.
		for attempts < 5 {
			if !g.ntripClient.Client.IsCasterAlive() {
				attempts++
				g.logger.Debugf("attempt(s) to connect to caster: %v ", attempts)
			} else {
				break
			}
		}
		if attempts == 5 {
			return fmt.Errorf("caster %s is down", g.ntripClient.URL)
		}
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

// connectToNTRIP connects to NTRIP stream.
func (g *gpsrtk) connectToNTRIP() error {
	select {
	case <-g.cancelCtx.Done():
		return errors.New("context canceled")
	default:
	}
	err := g.connectAndParseSourceTable()
	if err != nil {
		return err
	}

	g.reader, err = g.getStream()
	if err != nil {
		return err
	}
	return nil
}

func (g *gpsrtk) getStream() (io.Reader, error) {
	if g.isVirtualBase {
		g.logger.Debug("connecting to Virtual Reference Station")
		err := g.getNtripFromVRS()
		if err != nil {
			return nil, err
		}
		return io.TeeReader(g.vrs.ReaderWriter, g.correctionWriter), nil
	}
	g.logger.Debug("connecting to NTRIP stream........")
	err := g.getStreamFromMountPoint(g.ntripClient.MountPoint, g.ntripClient.MaxConnectAttempts)
	if err != nil {
		return nil, err
	}
	return io.TeeReader(g.ntripClient.Stream, g.correctionWriter), nil
}

// receiveAndWriteCorrectionData connects to the NTRIP receiver and sends the correction stream to
// the MovementSensor.
func (g *gpsrtk) receiveAndWriteCorrectionData() {
	defer g.activeBackgroundWorkers.Done()
	defer g.closePort()

	scanner := rtcm3.NewScanner(g.reader)

	for !g.isClosed {
		select {
		case <-g.cancelCtx.Done():
			return
		default:
		}

		// Calling NextMessage() reads from the scanner until a valid message is found, and returns
		// that. We don't care about the message: we care that the scanner is able to read messages
		// at all! So, focus on whether the scanner had errors (which indicate we need to reconnect
		// to the mount point), and not the message itself.
		_, err := scanner.NextMessage()
		if err == nil {
			continue // No errors: we're still connected.
		}

		// added a log so we do not always swallow the error
		g.logger.Debugf("no longer connected to NTRIP scanner: %s", err)

		if g.isClosed || g.cancelCtx.Err() != nil {
			return
		}

		// If we get here, the scanner encountered an error but is supposed to continue going. Try
		// reconnecting to the mount point.
		g.reader, err = g.getStream()
		if err != nil {
			g.err.Set(err)
			return
		}
		scanner = rtcm3.NewScanner(g.reader)
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

// readFix passthrough.
func (g *gpsrtk) readFix(ctx context.Context) (int, error) {
	lastError := g.err.Get()
	if lastError != nil {
		return 0, lastError
	}
	return g.cachedData.ReadFix(ctx)
}

// readSatsInView returns the number of satellites in view.
func (g *gpsrtk) readSatsInView(ctx context.Context) (int, error) {
	lastError := g.err.Get()
	if lastError != nil {
		return 0, lastError
	}

	return g.cachedData.ReadSatsInView(ctx)
}

// readSatsInUse returns the number of satellites in use.
func (g *gpsrtk) readSatsInUse(ctx context.Context) (int, error) {
	lastError := g.err.Get()
	if lastError != nil {
		return 0, lastError
	}

	return g.cachedData.ReadSatsInUse(ctx)
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

	fix, err := g.readFix(ctx)
	if err != nil {
		return nil, err
	}

	satsInView, err := g.readSatsInView(ctx)
	if err != nil {
		return nil, err
	}

	satsInUse, err := g.readSatsInUse(ctx)
	if err != nil {
		return nil, err
	}

	readings["fix"] = fix
	readings["satellites_in_view"] = satsInView
	readings["satellites_in_use"] = satsInUse

	return readings, nil
}

// Close shuts down the gpsrtk.
func (g *gpsrtk) Close(ctx context.Context) error {
	g.mu.Lock()
	g.cancelFunc()

	g.logger.Debug("Closing GPS RTK")
	if err := g.cachedData.Close(ctx); err != nil {
		g.mu.Unlock()
		return err
	}

	// close ntrip writer
	if g.correctionWriter != nil {
		if err := g.correctionWriter.Close(); err != nil {
			g.isClosed = true
			g.mu.Unlock()
			return err
		}
		g.correctionWriter = nil
	}

	// close ntrip client and stream
	if g.ntripClient.Client != nil {
		g.ntripClient.Client.CloseIdleConnections()
		g.ntripClient.Client = nil
	}

	if g.vrs != nil {
		if err := g.vrs.Close(); err != nil {
			g.mu.Unlock()
			return err
		}
	}

	if g.ntripClient.Stream != nil {
		if err := g.ntripClient.Stream.Close(); err != nil {
			g.mu.Unlock()
			return err
		}
		g.ntripClient.Stream = nil
	}

	g.mu.Unlock()
	g.activeBackgroundWorkers.Wait()

	if err := g.err.Get(); err != nil && !errors.Is(err, context.Canceled) {
		return err
	}

	g.logger.Debug("GPS RTK is closed")
	return nil
}

// getNtripFromVRS sends GGA messages to the NTRIP Caster over a TCP connection
// to get the NTRIP steam when the mount point is a Virtual Reference Station.
func (g *gpsrtk) getNtripFromVRS() error {
	g.mu.Lock()
	defer g.mu.Unlock()
	var err error
	if g.vrs != nil {
		if err := g.vrs.Close(); err != nil {
			return err
		}
		g.vrs = nil
	}
	g.vrs, err = gpsutils.ConnectToVirtualBase(g.cancelCtx, g.ntripClient, g.logger)
	if err != nil {
		return err
	}

	// read from the socket until we know if a successful connection has been
	// established.
	for {
		line, _, err := g.vrs.ReaderWriter.ReadLine()
		response := string(line)
		if err != nil {
			if errors.Is(err, io.EOF) {
				g.vrs.ReaderWriter = nil
				return err
			}
			g.logger.Error("Failed to read server response:", err)
			return err
		}

		if strings.HasPrefix(response, "HTTP/1.1 ") {
			if strings.Contains(response, "200 OK") {
				g.logger.Debug("Successful connection established with NTRIP caster.")
				break
			}
			g.logger.Errorf("Bad HTTP response: %v", response)
			return fmt.Errorf("server responded with non-OK status: %s", response)
		}
	}

	// We currently only write the GGA message when we try to reconnect to VRS. Some documentation for VRS states that we
	// should try to send a GGA message every 5-60 seconds, but more testing is needed to determine if that is required.
	// get the GGA message from cached data
	ggaMessage, err := g.cachedData.GGA()
	if err != nil {
		g.logger.Error("Failed to get GGA message")
		return err
	}

	g.logger.Debugf("Writing GGA message: %v\n", ggaMessage)

	_, err = g.vrs.ReaderWriter.WriteString(ggaMessage)
	if err != nil {
		g.logger.Error("Failed to send NMEA data:", err)
		return err
	}

	err = g.vrs.ReaderWriter.Flush()
	if err != nil {
		g.logger.Error("failed to write to buffer: ", err)
		return err
	}

	g.logger.Debug("GGA message sent successfully.")

	g.vrs.StartGGAThread(g.cachedData.GGA)
	return nil
}
