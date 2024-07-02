//go:build linux

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
			"i2c_bus": "1",
			"i2c_addr": 66,
			"i2c_baud_rate": 115200,
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

	"github.com/go-gnss/rtcm/rtcm3"
	"github.com/golang/geo/r3"
	geo "github.com/kellydunn/golang-geo"
	"go.viam.com/utils"

	"go.viam.com/rdk/components/board/genericlinux/buses"
	"go.viam.com/rdk/components/movementsensor"
	"go.viam.com/rdk/components/movementsensor/gpsutils"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/spatialmath"
)

var rtkmodel = resource.DefaultModelFamily.WithModel("gps-nmea-rtk-pmtk")

// Config is used for converting NMEA MovementSensor with RTK capabilities config attributes.
type Config struct {
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
	err := cfg.validateI2C(path)
	if err != nil {
		return nil, err
	}

	err = cfg.validateNtrip(path)
	if err != nil {
		return nil, err
	}

	return []string{}, nil
}

// validateI2C ensures all parts of the config are valid.
func (cfg *Config) validateI2C(path string) error {
	if cfg.I2CBus == "" {
		return resource.NewConfigValidationFieldRequiredError(path, "i2c_bus")
	}
	if cfg.I2CAddr == 0 {
		return resource.NewConfigValidationFieldRequiredError(path, "i2c_addr")
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
			Constructor: newRTKI2C,
		})
}

// rtkI2C is an nmea movementsensor model that can intake RTK correction data via I2C.
type rtkI2C struct {
	resource.Named
	resource.AlwaysRebuild
	logger     logging.Logger
	cancelCtx  context.Context
	cancelFunc func()

	activeBackgroundWorkers sync.WaitGroup

	mu          sync.Mutex
	ntripClient *gpsutils.NtripInfo

	err          movementsensor.LastError
	lastposition movementsensor.LastPosition

	cachedData       *gpsutils.CachedData
	correctionWriter io.ReadWriteCloser

	bus     buses.I2C
	mockI2c buses.I2C // Will be nil unless we're in a unit test
	wbaud   int
	addr    byte
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

	if g.mockI2c == nil {
		i2cbus, err := buses.NewI2cBus(newConf.I2CBus)
		if err != nil {
			return fmt.Errorf("gps init: failed to find i2c bus %s: %w", newConf.I2CBus, err)
		}
		g.bus = i2cbus
	} else {
		g.bus = g.mockI2c
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

	if g.ntripClient == nil {
		g.ntripClient = tempNtripClient
	} else {
		tempNtripClient.Client = g.ntripClient.Client
		tempNtripClient.Stream = g.ntripClient.Stream

		g.ntripClient = tempNtripClient
	}

	g.logger.CDebug(ctx, "done reconfiguring")

	return nil
}

func newRTKI2C(
	ctx context.Context,
	deps resource.Dependencies,
	conf resource.Config,
	logger logging.Logger,
) (movementsensor.MovementSensor, error) {
	return makeRTKI2C(ctx, deps, conf, logger, nil)
}

// makeRTKI2C is separate from newRTKI2C, above, so we can pass in a non-nil mock I2C bus during
// unit tests.
func makeRTKI2C(
	ctx context.Context,
	deps resource.Dependencies,
	conf resource.Config,
	logger logging.Logger,
	mockI2c buses.I2C,
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
		mockI2c:      mockI2c,
	}

	if err = g.Reconfigure(ctx, deps, conf); err != nil {
		return nil, err
	}

	config := gpsutils.I2CConfig{
		I2CBus:      newConf.I2CBus,
		I2CBaudRate: newConf.I2CBaudRate,
		I2CAddr:     newConf.I2CAddr,
	}
	if config.I2CBaudRate == 0 {
		config.I2CBaudRate = 115200
	}

	// If we have a mock I2C bus, pass that in, too. If we don't, it'll be nil and constructing the
	// reader will create a real I2C bus instead.
	dev, err := gpsutils.NewI2cDataReader(config, mockI2c, logger)
	if err != nil {
		return nil, err
	}
	g.cachedData = gpsutils.NewCachedData(dev, logger)

	if err := g.start(); err != nil {
		return nil, err
	}
	return g, g.err.Get()
}

// Start begins NTRIP receiver with i2c protocol and begins reading/updating MovementSensor measurements.
func (g *rtkI2C) start() error {
	g.activeBackgroundWorkers.Add(1)
	utils.PanicCapturingGo(func() { g.receiveAndWriteI2C(g.cancelCtx) })

	return g.err.Get()
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
			g.mu.Lock()
			defer g.mu.Unlock()
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
		} else {
			g.logger.Errorf("Can't connect to NTRIP stream: %s", err)
			return err
		}
	}

	g.logger.Debug("Connected to stream")
	g.mu.Lock()
	defer g.mu.Unlock()

	g.ntripClient.Stream = rc
	return g.err.Get()
}

// receiveAndWriteI2C connects to NTRIP receiver and sends correction stream to the MovementSensor through I2C protocol.
func (g *rtkI2C) receiveAndWriteI2C(ctx context.Context) {
	defer g.activeBackgroundWorkers.Done()
	if err := g.cancelCtx.Err(); err != nil {
		return
	}
	err := g.ntripClient.Connect(g.cancelCtx, g.logger)
	if err != nil {
		g.err.Set(err)
		return
	}

	if !g.ntripClient.Client.IsCasterAlive() {
		g.logger.CInfof(ctx, "caster %s seems to be down", g.ntripClient.URL)
	}

	// establish I2C connection
	handle, err := g.bus.OpenHandle(g.addr)
	if err != nil {
		g.logger.CErrorf(ctx, "can't open gps i2c %s", err)
		g.err.Set(err)
		return
	}
	defer utils.UncheckedErrorFunc(handle.Close)

	// Send GLL, RMC, VTG, GGA, GSA, and GSV sentences each 1000ms
	baudcmd := fmt.Sprintf("PMTK251,%d", g.wbaud)
	cmd251 := movementsensor.PMTKAddChk([]byte(baudcmd))
	cmd314 := movementsensor.PMTKAddChk([]byte("PMTK314,1,1,1,1,1,1,0,0,0,0,0,0,0,0,0,0,0,0,0"))
	cmd220 := movementsensor.PMTKAddChk([]byte("PMTK220,1000"))

	err = handle.Write(ctx, cmd251)
	if err != nil {
		g.logger.CDebug(ctx, "Failed to set baud rate")
	}

	err = handle.Write(ctx, cmd314)
	if err != nil {
		g.logger.CDebug(ctx, "failed to set NMEA output")
		g.err.Set(err)
		return
	}

	err = handle.Write(ctx, cmd220)
	if err != nil {
		g.logger.CDebug(ctx, "failed to set NMEA update rate")
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
		g.logger.CErrorf(ctx, "i2c handle write failed %s", err)
		g.err.Set(err)
		return
	}

	scanner := rtcm3.NewScanner(r)

	for {
		select {
		case <-g.cancelCtx.Done():
			g.err.Set(err)
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

		// If we get here, the scanner encountered an error but is supposed to continue going. Try
		// reconnecting to the mount point.
		g.logger.CDebug(ctx, "No message... reconnecting to stream...")
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
			g.logger.CErrorf(ctx, "i2c handle write failed %s", err)
			g.err.Set(err)
			return
		}

		scanner = rtcm3.NewScanner(r)
	}
}

// Position returns the current geographic location of the MOVEMENTSENSOR.
func (g *rtkI2C) Position(ctx context.Context, extra map[string]interface{}) (*geo.Point, float64, error) {
	g.mu.Lock()
	lastError := g.err.Get()
	if lastError != nil {
		lastPosition := g.lastposition.GetLastPosition()
		g.mu.Unlock()
		if lastPosition != nil {
			return lastPosition, 0, nil
		}
		return geo.NewPoint(math.NaN(), math.NaN()), math.NaN(), lastError
	}
	g.mu.Unlock()

	position, alt, err := g.cachedData.Position(ctx, extra)
	if err != nil {
		// Use the last known valid position if current position is (0,0)/ NaN.
		if position != nil && (movementsensor.IsZeroPosition(position) || movementsensor.IsPositionNaN(position)) {
			lastPosition := g.lastposition.GetLastPosition()
			if lastPosition != nil {
				return lastPosition, alt, nil
			}
		}
		return geo.NewPoint(math.NaN(), math.NaN()), math.NaN(), err
	}

	if movementsensor.IsPositionNaN(position) {
		position = g.lastposition.GetLastPosition()
	}

	return position, alt, nil
}

// LinearVelocity passthrough.
func (g *rtkI2C) LinearVelocity(ctx context.Context, extra map[string]interface{}) (r3.Vector, error) {
	g.mu.Lock()
	lastError := g.err.Get()
	if lastError != nil {
		defer g.mu.Unlock()
		return r3.Vector{}, lastError
	}
	g.mu.Unlock()

	return g.cachedData.LinearVelocity(ctx, extra)
}

// LinearAcceleration passthrough.
func (g *rtkI2C) LinearAcceleration(ctx context.Context, extra map[string]interface{}) (r3.Vector, error) {
	lastError := g.err.Get()
	if lastError != nil {
		return r3.Vector{}, lastError
	}
	return g.cachedData.LinearAcceleration(ctx, extra)
}

// AngularVelocity passthrough.
func (g *rtkI2C) AngularVelocity(ctx context.Context, extra map[string]interface{}) (spatialmath.AngularVelocity, error) {
	g.mu.Lock()
	lastError := g.err.Get()
	if lastError != nil {
		defer g.mu.Unlock()
		return spatialmath.AngularVelocity{}, lastError
	}
	g.mu.Unlock()

	return g.cachedData.AngularVelocity(ctx, extra)
}

// CompassHeading passthrough.
func (g *rtkI2C) CompassHeading(ctx context.Context, extra map[string]interface{}) (float64, error) {
	g.mu.Lock()
	lastError := g.err.Get()
	if lastError != nil {
		defer g.mu.Unlock()
		return 0, lastError
	}
	g.mu.Unlock()

	return g.cachedData.CompassHeading(ctx, extra)
}

// Orientation passthrough.
func (g *rtkI2C) Orientation(ctx context.Context, extra map[string]interface{}) (spatialmath.Orientation, error) {
	g.mu.Lock()
	lastError := g.err.Get()
	if lastError != nil {
		defer g.mu.Unlock()
		return spatialmath.NewZeroOrientation(), lastError
	}
	g.mu.Unlock()

	return g.cachedData.Orientation(ctx, extra)
}

// readFix passthrough.
func (g *rtkI2C) readFix(ctx context.Context) (int, error) {
	g.mu.Lock()
	lastError := g.err.Get()
	if lastError != nil {
		defer g.mu.Unlock()
		return 0, lastError
	}
	g.mu.Unlock()

	return g.cachedData.ReadFix(ctx)
}

func (g *rtkI2C) readSatsInView(ctx context.Context) (int, error) {
	g.mu.Lock()
	lastError := g.err.Get()
	if lastError != nil {
		defer g.mu.Unlock()
		return 0, lastError
	}
	g.mu.Unlock()

	return g.cachedData.ReadSatsInView(ctx)
}

// Properties passthrough.
func (g *rtkI2C) Properties(ctx context.Context, extra map[string]interface{}) (*movementsensor.Properties, error) {
	lastError := g.err.Get()
	if lastError != nil {
		return &movementsensor.Properties{}, lastError
	}

	return g.cachedData.Properties(ctx, extra)
}

// Accuracy passthrough.
func (g *rtkI2C) Accuracy(ctx context.Context, extra map[string]interface{}) (*movementsensor.Accuracy, error) {
	lastError := g.err.Get()
	if lastError != nil {
		return nil, lastError
	}

	return g.cachedData.Accuracy(ctx, extra)
}

// Readings will use the default MovementSensor Readings if not provided.
func (g *rtkI2C) Readings(ctx context.Context, extra map[string]interface{}) (map[string]interface{}, error) {
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

// Close shuts down the rtkI2C.
func (g *rtkI2C) Close(ctx context.Context) error {
	g.mu.Lock()
	g.cancelFunc()

	if err := g.cachedData.Close(ctx); err != nil {
		g.mu.Unlock()
		return err
	}

	// close ntrip writer
	if g.correctionWriter != nil {
		if err := g.correctionWriter.Close(); err != nil {
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

	return nil
}
