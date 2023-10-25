// Package gpsrtkpmtk implements a gps using serial connection
package gpsrtkpmtk

/*
	This package supports GPS RTK (Real Time Kinematics), which takes in the normal signals
	from the GNSS (Global Navigation Satellite Systems) along with a correction stream to achieve
	positional accuracy (accuracy tbd), over I2C bus.

	Example GPS RTK chip datasheet:
	https://content.u-blox.com/sites/default/files/ZED-F9P-04B_DataSheet_UBX-21044850.pdf

	Example configuration:

	{
		"name": "my-gps-rtk",
		"type": "movement_sensor",
		"model": "gps-nmea-rtk-pmtk",
		"attributes": {
			"board": "local",
			"i2c_addr": 66,
			"i2c_baud_rate": 115200,
			"i2c_bus": "default_bus",
			"ntrip_connect_attempts": 12,
			"ntrip_mountpoint": "MNTPT",
			"ntrip_password": "pass",
			"ntrip_url": "http://ntrip/url",
			"ntrip_username": "usr"
		},
		"depends_on": [],
	}

*/

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"strings"
	"sync"

	"github.com/de-bkg/gognss/pkg/ntrip"
	"github.com/edaniels/golog"
	"github.com/go-gnss/rtcm/rtcm3"
	"github.com/golang/geo/r3"
	geo "github.com/kellydunn/golang-geo"
	"go.viam.com/utils"

	"go.viam.com/rdk/components/board"
	"go.viam.com/rdk/components/movementsensor"
	gpsnmea "go.viam.com/rdk/components/movementsensor/gpsnmea"
	rtk "go.viam.com/rdk/components/movementsensor/rtkutils"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/spatialmath"
)

var rtkmodel = resource.DefaultModelFamily.WithModel("gps-nmea-rtk-pmtk")

const i2cStr = "i2c"

// Config is used for converting NMEA MovementSensor with RTK capabilities config attributes.
type Config struct {
	Board       string `json:"board"`
	I2CBus      string `json:"i2c_bus"`
	I2CAddr     int    `json:"i2c_addr"`
	I2CBaudRate int    `json:"i2c_baud_rate,omitempty"`

	NtripURL             string `json:"ntrip_url"`
	NtripConnectAttempts int    `json:"ntrip_connect_attempts,omitempty"`
	NtripMountpoint      string `json:"ntrip_mountpoint,omitempty"`
	NtripPass            string `json:"ntrip_password,omitempty"`
	NtripUser            string `json:"ntrip_username,omitempty"`
}

// Validate ensures all parts of the config are valid.
func (cfg *Config) Validate(path string) ([]string, error) {
	var deps []string

	err := cfg.validateI2C(path)
	if err != nil {
		return nil, err
	}

	err = cfg.validateNtrip(path)
	if err != nil {
		return nil, err
	}

	deps = append(deps, cfg.Board)
	return deps, nil
}

// validateI2C ensures all parts of the config are valid.
func (cfg *Config) validateI2C(path string) error {
	if cfg.I2CBus == "" {
		return utils.NewConfigValidationFieldRequiredError(path, "i2c_bus")
	}
	if cfg.I2CAddr == 0 {
		return utils.NewConfigValidationFieldRequiredError(path, "i2c_addr")
	}
	return nil
}

// validateNtrip ensures all parts of the config are valid.
func (cfg *Config) validateNtrip(path string) error {
	if cfg.NtripURL == "" {
		return utils.NewConfigValidationFieldRequiredError(path, "ntrip_url")
	}
	return nil
}

func init() {
	resource.RegisterComponent(
		movementsensor.API,
		rtkmodel,
		resource.Registration[movementsensor.MovementSensor, *Config]{
			Constructor: newRTKI2C,
		})
}

// rtkI2C is an nmea movementsensor model that can intake RTK correction data via I2C.
type rtkI2C struct {
	resource.Named
	resource.AlwaysRebuild
	logger     golog.Logger
	cancelCtx  context.Context
	cancelFunc func()

	activeBackgroundWorkers sync.WaitGroup

	mu               sync.Mutex
	ntripMu          sync.Mutex
	ntripconfigMu    sync.Mutex
	ntripClient      *rtk.NtripInfo
	connectedToNtrip bool

	err          movementsensor.LastError
	lastposition movementsensor.LastPosition

	nmeamovementsensor gpsnmea.NmeaMovementSensor
	correctionWriter   io.ReadWriteCloser

	bus   board.I2C
	wbaud int
	addr  byte
}

// Reconfigure reconfigures attributes.
func (g *rtkI2C) Reconfigure(ctx context.Context, deps resource.Dependencies, conf resource.Config) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	newConf, err := resource.NativeConfig[*Config](conf)
	if err != nil {
		return err
	}

	if newConf.I2CBaudRate == 0 {
		g.wbaud = 115200
	} else {
		g.wbaud = newConf.I2CBaudRate
	}

	g.addr = byte(newConf.I2CAddr)

	b, err := board.FromDependencies(deps, newConf.Board)
	if err != nil {
		return fmt.Errorf("gps init: failed to find board: %w", err)
	}
	localB, ok := b.(board.LocalBoard)
	if !ok {
		return fmt.Errorf("board %s is not local", newConf.Board)
	}

	i2cbus, ok := localB.I2CByName(newConf.I2CBus)
	if !ok {
		return fmt.Errorf("gps init: failed to find i2c bus %s", newConf.I2CBus)
	}
	g.bus = i2cbus

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

	g.logger.Debug("done reconfiguring")

	return nil
}

func newRTKI2C(
	ctx context.Context,
	deps resource.Dependencies,
	conf resource.Config,
	logger golog.Logger,
) (movementsensor.MovementSensor, error) {
	newConf, err := resource.NativeConfig[*Config](conf)
	if err != nil {
		return nil, err
	}

	cancelCtx, cancelFunc := context.WithCancel(context.Background())
	g := &rtkI2C{
		Named:        conf.ResourceName().AsNamed(),
		cancelCtx:    cancelCtx,
		cancelFunc:   cancelFunc,
		logger:       logger,
		err:          movementsensor.NewLastError(1, 1),
		lastposition: movementsensor.NewLastPosition(),
	}

	// reconfigure
	if err = g.Reconfigure(ctx, deps, conf); err != nil {
		return nil, err
	}

	nmeaConf := &gpsnmea.Config{
		ConnectionType: i2cStr,
	}

	// Init NMEAMovementSensor
	nmeaConf.I2CConfig = &gpsnmea.I2CConfig{
		Board:       newConf.Board,
		I2CBus:      newConf.I2CBus,
		I2CBaudRate: newConf.I2CBaudRate,
		I2CAddr:     newConf.I2CAddr,
	}

	if nmeaConf.I2CConfig.I2CBaudRate == 0 {
		nmeaConf.I2CConfig.I2CBaudRate = 115200
	}

	g.nmeamovementsensor, err = gpsnmea.NewPmtkI2CGPSNMEA(ctx, deps, conf.ResourceName(), nmeaConf, logger)
	if err != nil {
		return nil, err
	}

	if err := g.start(); err != nil {
		return nil, err
	}

	return g, g.err.Get()
}

// Start begins NTRIP receiver with i2c protocol and begins reading/updating MovementSensor measurements.
func (g *rtkI2C) start() error {
	// TODO(RDK-1639): Test out what happens if we call this line and then the ReceiveAndWrite*
	// correction data goes wrong. Could anything worse than uncorrected data occur?

	if err := g.nmeamovementsensor.Start(g.cancelCtx); err != nil {
		g.lastposition.GetLastPosition()
		return err
	}

	g.activeBackgroundWorkers.Add(1)
	utils.PanicCapturingGo(func() { g.receiveAndWriteI2C(g.cancelCtx) })

	return g.err.Get()
}

// connect attempts to connect to ntrip client until successful connection or timeout.
func (g *rtkI2C) connect(casterAddr, user, pwd string, maxAttempts int) error {
	g.logger.Info("starting connect")
	for attempts := 0; attempts < maxAttempts; attempts++ {
		ntripclient, err := ntrip.NewClient(casterAddr, ntrip.Options{Username: user, Password: pwd})
		if err == nil {
			g.logger.Debug("Connected to NTRIP caster")
			g.ntripMu.Lock()
			g.ntripClient.Client = ntripclient
			g.ntripMu.Unlock()
			return g.err.Get()
		}
	}

	errMsg := fmt.Sprintf("Can't connect to NTRIP caster after %d attempts", maxAttempts)
	return errors.New(errMsg)
}

// getStream attempts to connect to ntrip stream until successful connection or timeout.
func (g *rtkI2C) getStream(mountPoint string, maxAttempts int) error {
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

// receiveAndWriteI2C connects to NTRIP receiver and sends correction stream to the MovementSensor through I2C protocol.
func (g *rtkI2C) receiveAndWriteI2C(ctx context.Context) {
	defer g.activeBackgroundWorkers.Done()
	if err := g.cancelCtx.Err(); err != nil {
		return
	}
	err := g.connect(g.ntripClient.URL, g.ntripClient.Username, g.ntripClient.Password, g.ntripClient.MaxConnectAttempts)
	if err != nil {
		g.err.Set(err)
		return
	}

	if !g.ntripClient.Client.IsCasterAlive() {
		g.logger.Infof("caster %s seems to be down", g.ntripClient.URL)
	}

	srcTable, err := g.ntripClient.Client.ParseSourcetable()
	if err != nil {
		g.logger.Errorf("failed to get source table: %v", err)
	}
	isVirtualBase, nmeaerr := findLineWithMountPoint(srcTable, g.ntripClient.MountPoint)
	if nmeaerr != nil {
		g.logger.Errorf("can't find mountpoint in source table, found err %v\n", nmeaerr)
	}

	// establish I2C connection
	handle, err := g.bus.OpenHandle(g.addr)
	if err != nil {
		g.logger.Errorf("can't open gps i2c %s", err)
		g.err.Set(err)
		return
	}

	// Send GLL, RMC, VTG, GGA, GSA, and GSV sentences each 1000ms
	baudcmd := fmt.Sprintf("PMTK251,%d", g.wbaud)
	cmd251 := movementsensor.PMTKAddChk([]byte(baudcmd))
	cmd314 := movementsensor.PMTKAddChk([]byte("PMTK314,1,1,1,1,1,0,0,0,0,0,0,0,0,0,0,0,0,0,0"))
	cmd220 := movementsensor.PMTKAddChk([]byte("PMTK220,1000"))

	err = handle.Write(ctx, cmd251)
	if err != nil {
		g.logger.Debug("Failed to set baud rate")
	}

	err = handle.Write(ctx, cmd314)
	if err != nil {
		g.logger.Debug("failed to set NMEA output")
		g.err.Set(err)
		return
	}

	err = handle.Write(ctx, cmd220)
	if err != nil {
		g.logger.Debug("failed to set NMEA update rate")
		g.err.Set(err)
		return
	}

	err = g.getStream(g.ntripClient.MountPoint, g.ntripClient.MaxConnectAttempts)
	if err != nil {
		g.err.Set(err)
		return
	}

	// create a buffer
	w := &bytes.Buffer{}
	r := io.TeeReader(g.ntripClient.Stream, w)

	buf := make([]byte, 1100)
	n, err := g.ntripClient.Stream.Read(buf)
	if err != nil {
		g.err.Set(err)
		return
	}

	wI2C := movementsensor.PMTKAddChk(buf[:n])

	// port still open
	err = handle.Write(ctx, wI2C)
	if err != nil {
		g.logger.Errorf("i2c handle write failed %s", err)
		g.err.Set(err)
		return
	}

	scanner := rtcm3.NewScanner(r)

	g.ntripMu.Lock()
	g.connectedToNtrip = true
	g.ntripMu.Unlock()

	// It's okay to skip the mutex on this next line: g.connectedToNtrip can only be mutated by this
	// goroutine itself.
	for g.connectedToNtrip {
		select {
		case <-g.cancelCtx.Done():
			g.err.Set(err)
			return
		default:
		}

		// if we are dealing with a Virtual Base, we need to send GGA messages to
		// the caster in order to get NTRIP stream.
		if isVirtualBase {
			g.sendGGAMessage()
		}

		// establish I2C connection
		handle, err := g.bus.OpenHandle(g.addr)
		if err != nil {
			g.logger.Errorf("can't open gps i2c %s", err)
			g.err.Set(err)
			return
		}

		msg, err := scanner.NextMessage()
		if err != nil {
			g.ntripMu.Lock()
			g.connectedToNtrip = false
			g.ntripMu.Unlock()

			if msg == nil {
				g.logger.Debug("No message... reconnecting to stream...")
				err = g.getStream(g.ntripClient.MountPoint, g.ntripClient.MaxConnectAttempts)
				if err != nil {
					g.err.Set(err)
					return
				}

				w = &bytes.Buffer{}
				r = io.TeeReader(g.ntripClient.Stream, w)

				buf = make([]byte, 1100)
				n, err := g.ntripClient.Stream.Read(buf)
				if err != nil {
					g.err.Set(err)
					return
				}
				wI2C := movementsensor.PMTKAddChk(buf[:n])

				err = handle.Write(ctx, wI2C)

				if err != nil {
					g.logger.Errorf("i2c handle write failed %s", err)
					g.err.Set(err)
					return
				}

				scanner = rtcm3.NewScanner(r)
				g.ntripMu.Lock()
				g.connectedToNtrip = true
				g.ntripMu.Unlock()
				continue
			}
		}
		// close I2C
		err = handle.Close()
		if err != nil {
			g.logger.Debug("failed to close handle: %s", err)
			g.err.Set(err)
			return
		}
	}
}

// nolint
// getNtripConnectionStatus returns true if connection to NTRIP stream is OK, false if not
func (g *rtkI2C) getNtripConnectionStatus() (bool, error) {
	g.ntripMu.Lock()
	defer g.ntripMu.Unlock()
	return g.connectedToNtrip, g.err.Get()
}

// Position returns the current geographic location of the MOVEMENTSENSOR.
func (g *rtkI2C) Position(ctx context.Context, extra map[string]interface{}) (*geo.Point, float64, error) {
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
func (g *rtkI2C) LinearVelocity(ctx context.Context, extra map[string]interface{}) (r3.Vector, error) {
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
func (g *rtkI2C) LinearAcceleration(ctx context.Context, extra map[string]interface{}) (r3.Vector, error) {
	lastError := g.err.Get()
	if lastError != nil {
		return r3.Vector{}, lastError
	}
	return g.nmeamovementsensor.LinearAcceleration(ctx, extra)
}

// AngularVelocity passthrough.
func (g *rtkI2C) AngularVelocity(ctx context.Context, extra map[string]interface{}) (spatialmath.AngularVelocity, error) {
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
func (g *rtkI2C) CompassHeading(ctx context.Context, extra map[string]interface{}) (float64, error) {
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
func (g *rtkI2C) Orientation(ctx context.Context, extra map[string]interface{}) (spatialmath.Orientation, error) {
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
func (g *rtkI2C) readFix(ctx context.Context) (int, error) {
	g.ntripMu.Lock()
	lastError := g.err.Get()
	if lastError != nil {
		defer g.ntripMu.Unlock()
		return 0, lastError
	}
	g.ntripMu.Unlock()

	return g.nmeamovementsensor.ReadFix(ctx)
}

// Properties passthrough.
func (g *rtkI2C) Properties(ctx context.Context, extra map[string]interface{}) (*movementsensor.Properties, error) {
	lastError := g.err.Get()
	if lastError != nil {
		return &movementsensor.Properties{}, lastError
	}

	return g.nmeamovementsensor.Properties(ctx, extra)
}

// Accuracy passthrough.
func (g *rtkI2C) Accuracy(ctx context.Context, extra map[string]interface{}) (map[string]float32, error) {
	lastError := g.err.Get()
	if lastError != nil {
		return map[string]float32{}, lastError
	}

	return g.nmeamovementsensor.Accuracy(ctx, extra)
}

// Readings will use the default MovementSensor Readings if not provided.
func (g *rtkI2C) Readings(ctx context.Context, extra map[string]interface{}) (map[string]interface{}, error) {
	readings, err := movementsensor.Readings(ctx, g, extra)
	if err != nil {
		return nil, err
	}

	fix, err := g.readFix(ctx)
	if err != nil {
		return nil, err
	}

	readings["fix"] = fix

	return readings, nil
}

// Close shuts down the RTKMOVEMENTSENSOR.
func (g *rtkI2C) Close(ctx context.Context) error {
	g.ntripMu.Lock()
	g.cancelFunc()

	if err := g.nmeamovementsensor.Close(ctx); err != nil {
		g.ntripMu.Unlock()
		return err
	}

	// close ntrip writer
	if g.correctionWriter != nil {
		if err := g.correctionWriter.Close(); err != nil {
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

	return nil
}

// sendGGAMessage sends GGA messages to the serial port. This is only used to get NTRIP stream
// from a virtual reference point.
func (g *rtkI2C) sendGGAMessage() {
	// Open an I2C handle to read NMEA messages
	handle, err := g.bus.OpenHandle(g.addr)
	if err != nil {
		g.logger.Errorf("can't open gps i2c handle: %s", err)
		g.err.Set(err)
		return
	}
	defer func() {
		if closeErr := handle.Close(); closeErr != nil {
			g.logger.Errorf("error while closing I2C handle after reading GGA message: %v", closeErr)
			g.err.Set(closeErr)
		}
	}()

	// Create a buffer to collect NMEA messages
	messageBuffer := make([]byte, 0, 1024)

	for !g.connectedToNtrip {
		select {
		case <-g.cancelCtx.Done():
			return
		default:
		}

		// Read from the I2C device
		buffer, err := handle.Read(g.cancelCtx, 1024)
		if err != nil {
			g.err.Set(err)
			return
		}

		// Concatenate the received data with the existing message buffer
		buffer = append(messageBuffer, buffer...)

		// Strip out newlines and 0xFF
		bufferStr := strings.ReplaceAll(strings.ReplaceAll(string(buffer), "\n", ""), "\xFF", "")

		// Split the buffer into complete messages
		pieces := strings.Split(bufferStr, "\r")

		// The last piece didn't end with a carriage return yet.
		messageBuffer = []byte(pieces[len(pieces)-1])

		// All other pieces are complete messages
		for _, message := range pieces[:len(pieces)-1] {
			if strings.Contains(message, "GGA") {
				// Send the GGA message to the NTRIP caster
				if _, err := g.correctionWriter.Write([]byte(message)); err != nil {
					g.logger.Errorf("error while sending GGA message to NTRIP caster: %v", err)
					g.err.Set(err)
					return
				}
			}
		}
	}
}

// findLineWithMountPoint parses the given source-table returns the nmea bool of the given mount point.
// TODO: RSDK-5462.
func findLineWithMountPoint(sourceTable *ntrip.Sourcetable, mountPoint string) (bool, error) {
	stream, isFound := sourceTable.HasStream(mountPoint)

	if !isFound {
		return false, fmt.Errorf("can not find mountpoint %s in sourcetable", mountPoint)
	}

	return stream.Nmea, nil
}
