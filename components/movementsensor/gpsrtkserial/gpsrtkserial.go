// Package gpsrtkserial implements a gps using serial connection
package gpsrtkserial

/*
	This package supports GPS RTK (Real Time Kinematics), which takes in the normal signals
	from the GNSS (Global Navigation Satellite Systems) along with a correction stream to achieve
	positional accuracy (accuracy tbd), over Serial.

	Example GPS RTK chip datasheet:
	https://content.u-blox.com/sites/default/files/ZED-F9P-04B_DataSheet_UBX-21044850.pdf

	Ntrip Documentation:
	https://gssc.esa.int/wp-content/uploads/2018/07/NtripDocumentation.pdf

	Example configuration:
	{
      "type": "movement_sensor",
	  "model": "gps-nmea-rtk-serial",
      "name": "my-gps-rtk"
      "attributes": {
        "ntrip_url": "url",
        "ntrip_username": "usr",
        "ntrip_connect_attempts": 10,
        "ntrip_mountpoint": "MTPT",
        "ntrip_password": "pwd",
		"serial_baud_rate": 115200,
        "serial_path": "serial-path"
      },
      "depends_on": [],
    }

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
	slib "github.com/jacobsa/go-serial/serial"
	geo "github.com/kellydunn/golang-geo"
	"go.viam.com/utils"

	"go.viam.com/rdk/components/movementsensor"
	"go.viam.com/rdk/components/movementsensor/gpsutils"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/spatialmath"
)

var rtkmodel = resource.DefaultModelFamily.WithModel("gps-nmea-rtk-serial")

const (
	serialStr = "serial"
	ntripStr  = "ntrip"
)

// Config is used for converting NMEA MovementSensor with RTK capabilities config attributes.
type Config struct {
	SerialPath     string `json:"serial_path"`
	SerialBaudRate int    `json:"serial_baud_rate,omitempty"`

	NtripURL             string `json:"ntrip_url"`
	NtripConnectAttempts int    `json:"ntrip_connect_attempts,omitempty"`
	NtripMountpoint      string `json:"ntrip_mountpoint,omitempty"`
	NtripPass            string `json:"ntrip_password,omitempty"`
	NtripUser            string `json:"ntrip_username,omitempty"`
}

// Validate ensures all parts of the config are valid.
func (cfg *Config) Validate(path string) ([]string, error) {
	if cfg.SerialPath == "" {
		return nil, resource.NewConfigValidationFieldRequiredError(path, "serial_path")
	}

	if cfg.NtripURL == "" {
		return nil, resource.NewConfigValidationFieldRequiredError(path, "ntrip_url")
	}

	return nil, nil
}

func init() {
	resource.RegisterComponent(
		movementsensor.API,
		rtkmodel,
		resource.Registration[movementsensor.MovementSensor, *Config]{
			Constructor: newRTKSerial,
		})
}

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

// Reconfigure reconfigures attributes.
func (g *rtkSerial) Reconfigure(ctx context.Context, deps resource.Dependencies, conf resource.Config) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	newConf, err := resource.NativeConfig[*Config](conf)
	if err != nil {
		return err
	}

	if newConf.SerialPath != "" {
		g.writePath = newConf.SerialPath
		g.logger.CInfof(ctx, "updated serial_path to #%v", newConf.SerialPath)
	}

	if newConf.SerialBaudRate != 0 {
		g.wbaud = newConf.SerialBaudRate
		g.logger.CInfof(ctx, "updated serial_baud_rate to %v", newConf.SerialBaudRate)
	} else {
		g.wbaud = 38400
		g.logger.CInfo(ctx, "serial_baud_rate using default baud rate 38400")
	}

	ntripConfig := &gpsutils.NtripConfig{
		NtripURL:             newConf.NtripURL,
		NtripUser:            newConf.NtripUser,
		NtripPass:            newConf.NtripPass,
		NtripMountpoint:      newConf.NtripMountpoint,
		NtripConnectAttempts: newConf.NtripConnectAttempts,
	}

	// Init ntripInfo from attributes
	tempNtripClient, err := gpsutils.NewNtripInfo(ntripConfig, g.logger)
	if err != nil {
		return err
	}

	if g.ntripClient != nil { // Copy over the old state
		tempNtripClient.Client = g.ntripClient.Client
		tempNtripClient.Stream = g.ntripClient.Stream
	}

	g.ntripClient = tempNtripClient

	g.logger.Debug("done reconfiguring")
	return nil
}

func newRTKSerial(
	ctx context.Context,
	deps resource.Dependencies,
	conf resource.Config,
	logger logging.Logger,
) (movementsensor.MovementSensor, error) {
	newConf, err := resource.NativeConfig[*Config](conf)
	if err != nil {
		return nil, err
	}

	cancelCtx, cancelFunc := context.WithCancel(context.Background())
	g := &rtkSerial{
		Named:      conf.ResourceName().AsNamed(),
		cancelCtx:  cancelCtx,
		cancelFunc: cancelFunc,
		logger:     logger,
		err:        movementsensor.NewLastError(1, 1),
	}

	if err := g.Reconfigure(ctx, deps, conf); err != nil {
		return nil, err
	}

	g.InputProtocol = serialStr

	serialConfig := &gpsutils.SerialConfig{
		SerialPath:     newConf.SerialPath,
		SerialBaudRate: newConf.SerialBaudRate,
	}
	dev, err := gpsutils.NewSerialDataReader(serialConfig, logger)
	if err != nil {
		return nil, err
	}
	g.cachedData = gpsutils.NewCachedData(dev, logger)

	if err := g.start(); err != nil {
		return nil, err
	}
	return g, g.err.Get()
}

func (g *rtkSerial) start() error {
	err := g.connectToNTRIP()
	if err != nil {
		return err
	}
	g.activeBackgroundWorkers.Add(1)
	utils.PanicCapturingGo(g.receiveAndWriteSerial)
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

// openPort opens the serial port for writing.
func (g *rtkSerial) openPort() error {
	options := slib.OpenOptions{
		PortName:        g.writePath,
		BaudRate:        uint(g.wbaud),
		DataBits:        8,
		StopBits:        1,
		MinimumReadSize: 1,
	}

	if err := g.cancelCtx.Err(); err != nil {
		return err
	}

	var err error
	g.correctionWriter, err = slib.Open(options)
	if err != nil {
		g.logger.Errorf("serial.Open: %v", err)
		return err
	}

	return nil
}

// closePort closes the serial port.
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

	err = g.openPort()
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

// receiveAndWriteSerial connects to NTRIP receiver and sends correction stream to the MovementSensor through serial.
func (g *rtkSerial) receiveAndWriteSerial() {
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
		_, err := scanner.NextMessage()
		if err == nil {
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

	g.logger.Debug("Closing GPS RTK Serial")
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

	g.logger.Debug("GPS RTK Serial is closed")
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
