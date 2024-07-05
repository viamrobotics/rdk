// Package gpsrtk implements a gps
package gpsrtk

/*
	This package supports GPS RTK (Real Time Kinematics), which takes in the normal signals
	from the GNSS (Global Navigation Satellite Systems) along with a correction stream to achieve
	positional accuracy (accuracy tbd). This file is the main implementation, agnostic of how we
	communicate with the chip.

	Example GPS RTK chip datasheet:
	https://content.u-blox.com/sites/default/files/ZED-F9P-04B_DataSheet_UBX-21044850.pdf

	Ntrip Documentation:
	https://gssc.esa.int/wp-content/uploads/2018/07/NtripDocumentation.pdf

*/

import (
	"bufio"
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

// rtkSerial is an nmea movementsensor model that can intake RTK correction data.
type rtkSerial struct {
	resource.Named
	resource.AlwaysRebuild
	logger     logging.Logger
	cancelCtx  context.Context
	cancelFunc func()

	activeBackgroundWorkers sync.WaitGroup

	err           movementsensor.LastError
	InputProtocol string
	isClosed      bool

	mu sync.Mutex

	// everything below this comment is protected by mu
	ntripClient      *gpsutils.NtripInfo
	cachedData       *gpsutils.CachedData
	correctionWriter io.ReadWriteCloser
	writePath        string
	wbaud            int
	isVirtualBase    bool
	readerWriter     *bufio.ReadWriter
	writer           io.Writer
	reader           io.Reader
}

func (g *rtkSerial) start() error {
	err := g.connectToNTRIP()
	if err != nil {
		return err
	}
	g.activeBackgroundWorkers.Add(1)
	utils.PanicCapturingGo(g.receiveAndWriteCorrectionData)
	return g.err.Get()
}

// getStream attempts to connect to ntrip stream. We give up after maxAttempts unsuccessful tries.
func (g *rtkSerial) getStream(mountPoint string, maxAttempts int) error {
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
func (g *rtkSerial) closePort() {
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
func (g *rtkSerial) connectAndParseSourceTable() error {
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
func (g *rtkSerial) connectToNTRIP() error {
	select {
	case <-g.cancelCtx.Done():
		return errors.New("context canceled")
	default:
	}
	err := g.connectAndParseSourceTable()
	if err != nil {
		return err
	}

	if g.isVirtualBase {
		g.logger.Debug("connecting to a Virtual Reference Station")
		err = g.getNtripFromVRS()
		if err != nil {
			return err
		}
	} else {
		g.logger.Debug("connecting to NTRIP stream........")
		g.writer = bufio.NewWriter(g.correctionWriter)
		err = g.getStream(g.ntripClient.MountPoint, g.ntripClient.MaxConnectAttempts)
		if err != nil {
			return err
		}

		g.reader = io.TeeReader(g.ntripClient.Stream, g.writer)
	}

	return nil
}

// receiveAndWriteCorrectionData connects to the NTRIP receiver and sends the correction stream to
// the MovementSensor.
func (g *rtkSerial) receiveAndWriteCorrectionData() {
	defer g.activeBackgroundWorkers.Done()
	defer g.closePort()

	var scanner rtcm3.Scanner

	if g.isVirtualBase {
		scanner = rtcm3.NewScanner(g.readerWriter)
	} else {
		scanner = rtcm3.NewScanner(g.reader)
	}

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
		msg, err := scanner.NextMessage()
		if err == nil {
			bytes := msg.Serialize()
			g.logger.Debugf("writing %d bytes to GPS", len(bytes))
			continue // No errors: we're still connected.
		}

		if g.isClosed {
			return
		}

		// If we get here, the scanner encountered an error but is supposed to continue going. Try
		// reconnecting to the mount point.
		if g.isVirtualBase {
			g.logger.Debug("reconnecting to the Virtual Reference Station")
			err = g.getNtripFromVRS()
			if err != nil && !errors.Is(err, io.EOF) {
				g.err.Set(err)
				return
			}
			scanner = rtcm3.NewScanner(g.readerWriter)
		} else {
			g.logger.Debug("No message... reconnecting to stream...")

			err = g.getStream(g.ntripClient.MountPoint, g.ntripClient.MaxConnectAttempts)
			if err != nil {
				g.err.Set(err)
				return
			}
			g.reader = io.TeeReader(g.ntripClient.Stream, g.writer)
			scanner = rtcm3.NewScanner(g.reader)
		}
	}
}

// Most of the movementsensor functions here don't have mutex locks since g.cachedData is protected by
// it's own mutex and not having mutex around g.err is alright.

// Position returns the current geographic location of the MOVEMENTSENSOR.
func (g *rtkSerial) Position(ctx context.Context, extra map[string]interface{}) (*geo.Point, float64, error) {
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
func (g *rtkSerial) LinearVelocity(ctx context.Context, extra map[string]interface{}) (r3.Vector, error) {
	lastError := g.err.Get()
	if lastError != nil {
		return r3.Vector{}, lastError
	}

	return g.cachedData.LinearVelocity(ctx, extra)
}

// LinearAcceleration passthrough.
func (g *rtkSerial) LinearAcceleration(ctx context.Context, extra map[string]interface{}) (r3.Vector, error) {
	lastError := g.err.Get()
	if lastError != nil {
		return r3.Vector{}, lastError
	}
	return g.cachedData.LinearAcceleration(ctx, extra)
}

// AngularVelocity passthrough.
func (g *rtkSerial) AngularVelocity(ctx context.Context, extra map[string]interface{}) (spatialmath.AngularVelocity, error) {
	lastError := g.err.Get()
	if lastError != nil {
		return spatialmath.AngularVelocity{}, lastError
	}

	return g.cachedData.AngularVelocity(ctx, extra)
}

// CompassHeading passthrough.
func (g *rtkSerial) CompassHeading(ctx context.Context, extra map[string]interface{}) (float64, error) {
	lastError := g.err.Get()
	if lastError != nil {
		return 0, lastError
	}
	return g.cachedData.CompassHeading(ctx, extra)
}

// Orientation passthrough.
func (g *rtkSerial) Orientation(ctx context.Context, extra map[string]interface{}) (spatialmath.Orientation, error) {
	lastError := g.err.Get()
	if lastError != nil {
		return spatialmath.NewZeroOrientation(), lastError
	}
	return g.cachedData.Orientation(ctx, extra)
}

// readFix passthrough.
func (g *rtkSerial) readFix(ctx context.Context) (int, error) {
	lastError := g.err.Get()
	if lastError != nil {
		return 0, lastError
	}
	return g.cachedData.ReadFix(ctx)
}

// readSatsInView returns the number of satellites in view.
func (g *rtkSerial) readSatsInView(ctx context.Context) (int, error) {
	lastError := g.err.Get()
	if lastError != nil {
		return 0, lastError
	}

	return g.cachedData.ReadSatsInView(ctx)
}

// Properties passthrough.
func (g *rtkSerial) Properties(ctx context.Context, extra map[string]interface{}) (*movementsensor.Properties, error) {
	lastError := g.err.Get()
	if lastError != nil {
		return &movementsensor.Properties{}, lastError
	}

	return g.cachedData.Properties(ctx, extra)
}

// Accuracy passthrough.
func (g *rtkSerial) Accuracy(ctx context.Context, extra map[string]interface{}) (*movementsensor.Accuracy, error,
) {
	lastError := g.err.Get()
	if lastError != nil {
		return nil, lastError
	}

	return g.cachedData.Accuracy(ctx, extra)
}

// Readings will use the default MovementSensor Readings if not provided.
func (g *rtkSerial) Readings(ctx context.Context, extra map[string]interface{}) (map[string]interface{}, error) {
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

	readings["fix"] = fix
	readings["satellites_in_view"] = satsInView

	return readings, nil
}

// Close shuts down the rtkSerial.
func (g *rtkSerial) Close(ctx context.Context) error {
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
func (g *rtkSerial) getNtripFromVRS() error {
	g.mu.Lock()
	defer g.mu.Unlock()
	var err error
	g.readerWriter, err = gpsutils.ConnectToVirtualBase(g.ntripClient, g.logger)
	if err != nil {
		return err
	}

	// read from the socket until we know if a successful connection has been
	// established.
	for {
		line, _, err := g.readerWriter.ReadLine()
		response := string(line)
		if err != nil {
			if errors.Is(err, io.EOF) {
				g.readerWriter = nil
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

	ggaMessage, err := gpsutils.GetGGAMessage(g.correctionWriter, g.logger)
	if err != nil {
		g.logger.Error("Failed to get GGA message")
		return err
	}

	g.logger.Debugf("Writing GGA message: %v\n", string(ggaMessage))

	_, err = g.readerWriter.WriteString(string(ggaMessage))
	if err != nil {
		g.logger.Error("Failed to send NMEA data:", err)
		return err
	}

	err = g.readerWriter.Flush()
	if err != nil {
		g.logger.Error("failed to write to buffer: ", err)
		return err
	}

	g.logger.Debug("GGA message sent successfully.")

	return nil
}
