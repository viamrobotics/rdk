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

	"github.com/de-bkg/gognss/pkg/ntrip"
	"github.com/go-gnss/rtcm/rtcm3"
	"github.com/golang/geo/r3"
	slib "github.com/jacobsa/go-serial/serial"
	geo "github.com/kellydunn/golang-geo"
	"go.viam.com/utils"

	"go.viam.com/rdk/components/movementsensor"
	gpsnmea "go.viam.com/rdk/components/movementsensor/gpsnmea"
	rtk "go.viam.com/rdk/components/movementsensor/rtkutils"
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
	err := cfg.validateNtrip(path)
	if err != nil {
		return nil, err
	}

	err = cfg.validateSerialPath(path)
	if err != nil {
		return nil, err
	}

	return nil, nil
}

// validateSerialPath ensures all parts of the config are valid.
func (cfg *Config) validateSerialPath(path string) error {
	if cfg.SerialPath == "" {
		return resource.NewConfigValidationFieldRequiredError(path, "serial_path")
	}
	return nil
}

// validateNtrip ensures all parts of the config are valid.
func (cfg *Config) validateNtrip(path string) error {
	if cfg.NtripURL == "" {
		return resource.NewConfigValidationFieldRequiredError(path, "ntrip_url")
	}
	return nil
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

	mu                 sync.Mutex // Mutex for general synchronization during reconfigure.
	ntripMu            sync.Mutex // Mutex for NTRIP-related operations.
	ntripconfigMu      sync.Mutex // Mutex for NTRIP configuration.
	urlMutex           sync.Mutex // Mutex for URL-related operations.
	ntripClient        *rtk.NtripInfo
	isConnectedToNtrip bool
	isClosed           bool

	err                movementsensor.LastError
	lastposition       movementsensor.LastPosition
	lastcompassheading movementsensor.LastCompassHeading
	InputProtocol      string

	nmeamovementsensor gpsnmea.NmeaMovementSensor
	correctionWriter   io.ReadWriteCloser
	writePath          string
	wbaud              int
	isVirtualBase      bool
	readerWriter       *bufio.ReadWriter
	writer             io.Writer
	reader             io.Reader
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

	g.ntripconfigMu.Lock()
	ntripConfig := &rtk.NtripConfig{
		NtripURL:             newConf.NtripURL,
		NtripUser:            newConf.NtripUser,
		NtripPass:            newConf.NtripPass,
		NtripMountpoint:      newConf.NtripMountpoint,
		NtripConnectAttempts: newConf.NtripConnectAttempts,
	}

	// Init ntripInfo from attributes
	tempNtripClient, err := rtk.NewNtripInfo(ntripConfig, g.logger)
	if err != nil {
		return err
	}

	if g.ntripClient == nil {
		g.ntripClient = tempNtripClient
	} else {
		tempNtripClient.Client = g.ntripClient.Client
		tempNtripClient.Stream = g.ntripClient.Stream

		g.ntripClient = tempNtripClient
	}

	g.ntripconfigMu.Unlock()

	g.logger.CDebug(ctx, "done reconfiguring")

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
		Named:              conf.ResourceName().AsNamed(),
		cancelCtx:          cancelCtx,
		cancelFunc:         cancelFunc,
		logger:             logger,
		err:                movementsensor.NewLastError(1, 1),
		lastposition:       movementsensor.NewLastPosition(),
		lastcompassheading: movementsensor.NewLastCompassHeading(),
	}

	// reconfigure
	if err := g.Reconfigure(ctx, deps, conf); err != nil {
		return nil, err
	}

	g.InputProtocol = serialStr
	nmeaConf := &gpsnmea.Config{
		ConnectionType: serialStr,
	}

	// Init NMEAMovementSensor
	nmeaConf.SerialConfig = &gpsnmea.SerialConfig{
		SerialPath:     newConf.SerialPath,
		SerialBaudRate: newConf.SerialBaudRate,
	}
	g.nmeamovementsensor, err = gpsnmea.NewSerialGPSNMEA(ctx, conf.ResourceName(), nmeaConf, logger)
	if err != nil {
		return nil, err
	}

	if err := g.start(); err != nil {
		return nil, err
	}
	return g, g.err.Get()
}

func (g *rtkSerial) start() error {
	if err := g.nmeamovementsensor.Start(g.cancelCtx); err != nil {
		g.lastposition.GetLastPosition()
		return err
	}

	if !g.isClosed {
		err := g.connectToNTRIP()
		if err != nil {
			return err
		}
		g.activeBackgroundWorkers.Add(1)
		utils.PanicCapturingGo(g.receiveAndWriteSerial)
	}
	return g.err.Get()
}

// connect attempts to connect to ntrip client until successful connection or timeout.
func (g *rtkSerial) connect(casterAddr, user, pwd string, maxAttempts int) error {
	attempts := 0

	var c *ntrip.Client
	var err error

	g.logger.Debug("Connecting to NTRIP caster")
	for attempts < maxAttempts {
		select {
		case <-g.cancelCtx.Done():
			return g.cancelCtx.Err()
		default:
		}

		c, err = ntrip.NewClient(casterAddr, ntrip.Options{Username: user, Password: pwd})
		if err == nil {
			break
		}

		attempts++
	}

	if err != nil {
		g.logger.Errorf("Can't connect to NTRIP caster: %s", err)
		return err
	}

	g.logger.Info("Connected to NTRIP caster")
	g.ntripMu.Lock()
	g.ntripClient.Client = c
	g.ntripMu.Unlock()
	return g.err.Get()
}

// getStream attempts to connect to ntrip streak until successful connection or timeout.
func (g *rtkSerial) getStream(mountPoint string, maxAttempts int) error {
	success := false
	attempts := 0

	var rc io.ReadCloser
	var err error

	g.logger.Debug("Getting NTRIP stream")

	for !success && attempts < maxAttempts && !g.isClosed {
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
		// if the error is related to ICY, we log it as warning.
		if strings.Contains(err.Error(), "ICY") {
			g.logger.Warnf("Detected old HTTP protocol: %s", err)
			g.err.Set(err)
		} else {
			g.logger.Errorf("Can't connect to NTRIP stream: %s", err)
			return err
		}
	}

	if success {
		g.logger.Debug("Connected to stream")
	}

	g.ntripMu.Lock()
	defer g.ntripMu.Unlock()

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

	g.ntripMu.Lock()
	defer g.ntripMu.Unlock()

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
	g.ntripMu.Lock()
	defer g.ntripMu.Unlock()

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

	g.urlMutex.Lock()
	defer g.urlMutex.Unlock()

	err := g.connect(g.ntripClient.URL, g.ntripClient.Username, g.ntripClient.Password, g.ntripClient.MaxConnectAttempts)
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

	g.logger.Debug("gettting source table")
	srcTable, err := g.ntripClient.Client.ParseSourcetable()
	if err != nil {
		g.logger.Errorf("failed to get source table: %v", err)
		return err
	}
	g.logger.Debug("got sourcetable, parsing it...")
	g.isVirtualBase, err = rtk.FindLineWithMountPoint(srcTable, g.ntripClient.MountPoint)
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
		return g.err.Get()
	}

	err = g.openPort()
	if err != nil {
		g.err.Set(err)
		return g.err.Get()
	}

	if g.isVirtualBase {
		g.logger.Debug("connecting to a Virtual Reference Station")
		err = g.getNtripFromVRS()
		if err != nil {
			g.err.Set(err)
			return g.err.Get()
		}
	} else {
		g.logger.Debug("connecting to NTRIP stream........")
		g.writer = bufio.NewWriter(g.correctionWriter)
		err = g.getStream(g.ntripClient.MountPoint, g.ntripClient.MaxConnectAttempts)
		if err != nil {
			g.err.Set(err)
			return g.err.Get()
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

	g.ntripMu.Lock()
	g.isConnectedToNtrip = true
	g.ntripMu.Unlock()

	// It's okay to skip the mutex on this next line: g.isConnectedToNtrip can only be mutated by this
	// goroutine itself
	for g.isConnectedToNtrip && !g.isClosed {
		select {
		case <-g.cancelCtx.Done():
			return
		default:
		}

		msg, err := scanner.NextMessage()
		if err != nil {
			g.ntripMu.Lock()
			g.isConnectedToNtrip = false
			g.ntripMu.Unlock()

			if msg == nil {
				if g.isClosed {
					return
				}

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

				g.ntripMu.Lock()
				g.isConnectedToNtrip = true
				g.ntripMu.Unlock()
				continue
			}
		}
	}
}

// Position returns the current geographic location of the MOVEMENTSENSOR.
func (g *rtkSerial) Position(ctx context.Context, extra map[string]interface{}) (*geo.Point, float64, error) {
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

	position, alt, err := g.nmeamovementsensor.Position(ctx, extra)
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

	if g.lastposition.IsPositionNaN(position) {
		position = g.lastposition.GetLastPosition()
	}
	return position, alt, nil
}

// LinearVelocity passthrough.
func (g *rtkSerial) LinearVelocity(ctx context.Context, extra map[string]interface{}) (r3.Vector, error) {
	g.ntripMu.Lock()
	lastError := g.err.Get()
	if lastError != nil {
		defer g.ntripMu.Unlock()
		return r3.Vector{}, lastError
	}
	g.ntripMu.Unlock()

	return g.nmeamovementsensor.LinearVelocity(ctx, extra)
}

// LinearAcceleration passthrough.
func (g *rtkSerial) LinearAcceleration(ctx context.Context, extra map[string]interface{}) (r3.Vector, error) {
	lastError := g.err.Get()
	if lastError != nil {
		return r3.Vector{}, lastError
	}
	return g.nmeamovementsensor.LinearAcceleration(ctx, extra)
}

// AngularVelocity passthrough.
func (g *rtkSerial) AngularVelocity(ctx context.Context, extra map[string]interface{}) (spatialmath.AngularVelocity, error) {
	g.ntripMu.Lock()
	lastError := g.err.Get()
	if lastError != nil {
		defer g.ntripMu.Unlock()
		return spatialmath.AngularVelocity{}, lastError
	}
	g.ntripMu.Unlock()

	return g.nmeamovementsensor.AngularVelocity(ctx, extra)
}

// CompassHeading passthrough.
func (g *rtkSerial) CompassHeading(ctx context.Context, extra map[string]interface{}) (float64, error) {
	g.ntripMu.Lock()
	lastError := g.err.Get()
	if lastError != nil {
		defer g.ntripMu.Unlock()
		return 0, lastError
	}
	g.ntripMu.Unlock()
	return g.nmeamovementsensor.CompassHeading(ctx, extra)
}

// Orientation passthrough.
func (g *rtkSerial) Orientation(ctx context.Context, extra map[string]interface{}) (spatialmath.Orientation, error) {
	g.ntripMu.Lock()
	lastError := g.err.Get()
	if lastError != nil {
		defer g.ntripMu.Unlock()
		return spatialmath.NewZeroOrientation(), lastError
	}
	g.ntripMu.Unlock()
	return g.nmeamovementsensor.Orientation(ctx, extra)
}

// readFix passthrough.
func (g *rtkSerial) readFix(ctx context.Context) (int, error) {
	g.ntripMu.Lock()
	lastError := g.err.Get()
	if lastError != nil {
		defer g.ntripMu.Unlock()
		return 0, lastError
	}
	g.ntripMu.Unlock()
	return g.nmeamovementsensor.ReadFix(ctx)
}

// readSatsInView returns the number of satellites in view.
func (g *rtkSerial) readSatsInView(ctx context.Context) (int, error) {
	g.ntripMu.Lock()
	lastError := g.err.Get()
	if lastError != nil {
		defer g.ntripMu.Unlock()
		return 0, lastError
	}

	g.ntripMu.Unlock()
	return g.nmeamovementsensor.ReadSatsInView(ctx)
}

// Properties passthrough.
func (g *rtkSerial) Properties(ctx context.Context, extra map[string]interface{}) (*movementsensor.Properties, error) {
	lastError := g.err.Get()
	if lastError != nil {
		return &movementsensor.Properties{}, lastError
	}

	return g.nmeamovementsensor.Properties(ctx, extra)
}

// Accuracy passthrough.
func (g *rtkSerial) Accuracy(ctx context.Context, extra map[string]interface{}) (map[string]float32, error) {
	lastError := g.err.Get()
	if lastError != nil {
		return map[string]float32{}, lastError
	}

	return g.nmeamovementsensor.Accuracy(ctx, extra)
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
	g.ntripMu.Lock()
	g.cancelFunc()

	g.logger.Debug("Closing GPS RTK Serial")
	if err := g.nmeamovementsensor.Close(ctx); err != nil {
		g.ntripMu.Unlock()
		return err
	}

	// close ntrip writer
	if g.correctionWriter != nil {
		if err := g.correctionWriter.Close(); err != nil {
			g.isClosed = true
			g.ntripMu.Unlock()
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

	g.logger.Debug("GPS RTK Serial is closed")
	return nil
}

// getNtripFromVRS sends GGA messages to the NTRIP Caster over a TCP connection
// to get the NTRIP steam when the mount point is a Virtual Reference Station.
func (g *rtkSerial) getNtripFromVRS() error {
	g.ntripMu.Lock()
	defer g.ntripMu.Unlock()

	g.readerWriter = rtk.ConnectToVirtualBase(g.ntripClient, g.logger)

	// read from the socket until we know if a successful connection has been
	// established.
	for {
		line, _, err := g.readerWriter.ReadLine()
		if err != nil {
			if errors.Is(err, io.EOF) {
				g.readerWriter = nil
				return err
			}
			g.logger.Error("Failed to read server response:", err)
			return err
		}

		if strings.HasPrefix(string(line), "HTTP/1.1 ") {
			if strings.Contains(string(line), "200 OK") {
				break
			} else {
				g.logger.Errorf("Bad HTTP response: %v", string(line))
				return err
			}
		}
	}

	ggaMessage, err := rtk.GetGGAMessage(g.correctionWriter, g.logger)
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
