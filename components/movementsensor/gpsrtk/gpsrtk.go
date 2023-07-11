// Package gpsrtk defines a gps and an rtk correction source
// which sends rtcm data to a child gps
// This is an Experimental package
package gpsrtk

import (
	"bufio"
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
	slib "github.com/jacobsa/go-serial/serial"
	geo "github.com/kellydunn/golang-geo"
	"go.viam.com/utils"

	"go.viam.com/rdk/components/board"
	"go.viam.com/rdk/components/movementsensor"
	gpsnmea "go.viam.com/rdk/components/movementsensor/gpsnmea"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/spatialmath"
)

var (
	errCorrectionSourceValidation = fmt.Errorf("only serial, i2c, and ntrip are supported correction sources for %s", roverModel.Name)
	errConnectionTypeValidation   = fmt.Errorf("only serial and i2c are supported connection types for %s", roverModel.Name)
	errInputProtocolValidation    = fmt.Errorf("only serial and i2c are supported input protocols for %s", roverModel.Name)
)

var roverModel = resource.DefaultModelFamily.WithModel("gps-rtk")

const (
	i2cStr    = "i2c"
	serialStr = "serial"
	ntripStr  = "ntrip"
)

// Config is used for converting NMEA MovementSensor with RTK capabilities config attributes.
type Config struct {
	CorrectionSource string `json:"correction_source"`
	ConnectionType   string `json:"connection_type,omitempty"`

	*SerialConfig `json:"serial_attributes,omitempty"`
	*I2CConfig    `json:"i2c_attributes,omitempty"`
	*NtripConfig  `json:"ntrip_attributes,omitempty"`
}

// NtripConfig is used for converting attributes for a correction source.
type NtripConfig struct {
	NtripURL             string `json:"ntrip_url"`
	NtripConnectAttempts int    `json:"ntrip_connect_attempts,omitempty"`
	NtripMountpoint      string `json:"ntrip_mountpoint,omitempty"`
	NtripPass            string `json:"ntrip_password,omitempty"`
	NtripUser            string `json:"ntrip_username,omitempty"`
	NtripPath            string `json:"ntrip_path,omitempty"`
	NtripBaud            int    `json:"ntrip_baud,omitempty"`
	NtripInputProtocol   string `json:"ntrip_input_protocol,omitempty"`
}

// SerialConfig is used for converting attributes for a correction source.
type SerialConfig struct {
	SerialPath               string `json:"serial_path"`
	SerialBaudRate           int    `json:"serial_baud_rate,omitempty"`
	SerialCorrectionPath     string `json:"serial_correction_path,omitempty"`
	SerialCorrectionBaudRate int    `json:"serial_correction_baud_rate,omitempty"`

	// TestChan is a fake "serial" path for test use only
	TestChan chan []uint8 `json:"-"`
}

// I2CConfig is used for converting attributes for a correction source.
type I2CConfig struct {
	Board       string `json:"board"`
	I2CBus      string `json:"i2c_bus"`
	I2cAddr     int    `json:"i2c_addr"`
	I2CBaudRate int    `json:"i2c_baud_rate,omitempty"`
}

// Validate ensures all parts of the config are valid.
func (cfg *Config) Validate(path string) ([]string, error) {
	var deps []string

	dep, err := cfg.validateCorrectionSource(path)
	if err != nil {
		return nil, err
	}
	if dep != nil {
		deps = append(deps, dep...)
	}

	dep, err = cfg.validateConnectionType(path)
	if err != nil {
		return nil, err
	}
	if dep != nil {
		deps = append(deps, dep...)
	}

	if cfg.CorrectionSource == ntripStr {
		dep, err = cfg.validateNtripInputProtocol(path)
		if err != nil {
			return nil, err
		}
	}
	if dep != nil {
		deps = append(deps, dep...)
	}

	return deps, nil
}

func (cfg *Config) validateCorrectionSource(path string) ([]string, error) {
	var deps []string
	switch cfg.CorrectionSource {
	case ntripStr:
		return nil, cfg.NtripConfig.ValidateNtrip(path)
	case i2cStr:
		if cfg.Board == "" {
			return nil, utils.NewConfigValidationFieldRequiredError(path, "board")
		}
		deps = append(deps, cfg.Board)
		return deps, cfg.I2CConfig.ValidateI2C(path)
	case serialStr:
		return nil, cfg.SerialConfig.ValidateSerial(path)
	case "":
		return nil, utils.NewConfigValidationFieldRequiredError(path, "correction_source")
	default:
		return nil, errCorrectionSourceValidation
	}
}

func (cfg *Config) validateConnectionType(path string) ([]string, error) {
	var deps []string
	switch strings.ToLower(cfg.ConnectionType) {
	case i2cStr:
		if cfg.Board == "" {
			return nil, utils.NewConfigValidationFieldRequiredError(path, "board")
		}
		deps = append(deps, cfg.Board)
		return deps, cfg.I2CConfig.ValidateI2C(path)
	case serialStr:
		return nil, cfg.SerialConfig.ValidateSerial(path)
	case "":
		return nil, utils.NewConfigValidationFieldRequiredError(path, "connection_type")
	default:
		return nil, errConnectionTypeValidation
	}
}

func (cfg *Config) validateNtripInputProtocol(path string) ([]string, error) {
	var deps []string
	switch cfg.NtripInputProtocol {
	case i2cStr:
		if cfg.Board == "" {
			return nil, utils.NewConfigValidationFieldRequiredError(path, "board")
		}
		deps = append(deps, cfg.Board)
		return deps, cfg.I2CConfig.ValidateI2C(path)
	case serialStr:
		return nil, cfg.SerialConfig.ValidateSerial(path)
	default:
		return nil, errInputProtocolValidation
	}
}

// ValidateI2C ensures all parts of the config are valid.
func (cfg *I2CConfig) ValidateI2C(path string) error {
	if cfg.I2CBus == "" {
		return utils.NewConfigValidationFieldRequiredError(path, "i2c_bus")
	}
	if cfg.I2cAddr == 0 {
		return utils.NewConfigValidationFieldRequiredError(path, "i2c_addr")
	}
	return nil
}

// ValidateSerial ensures all parts of the config are valid.
func (cfg *SerialConfig) ValidateSerial(path string) error {
	if cfg.SerialPath == "" {
		return utils.NewConfigValidationFieldRequiredError(path, "serial_path")
	}
	return nil
}

// ValidateNtrip ensures all parts of the config are valid.
func (cfg *NtripConfig) ValidateNtrip(path string) error {
	if cfg.NtripURL == "" {
		return utils.NewConfigValidationFieldRequiredError(path, "ntrip_url")
	}
	if cfg.NtripInputProtocol == "" {
		return utils.NewConfigValidationFieldRequiredError(path, "ntrip_input_protocol")
	}
	return nil
}

func init() {
	resource.RegisterComponent(
		movementsensor.API,
		roverModel,
		resource.Registration[movementsensor.MovementSensor, *Config]{
			Constructor: newRTKMovementSensor,
		})
}

// A RTKMovementSensor is an NMEA MovementSensor model that can intake RTK correction data.
type RTKMovementSensor struct {
	resource.Named
	resource.AlwaysRebuild
	logger     golog.Logger
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

func newRTKMovementSensor(
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
	g := &RTKMovementSensor{
		Named:        conf.ResourceName().AsNamed(),
		cancelCtx:    cancelCtx,
		cancelFunc:   cancelFunc,
		logger:       logger,
		err:          movementsensor.NewLastError(1, 1),
		lastposition: movementsensor.NewLastPosition(),
	}

	if newConf.CorrectionSource == ntripStr {
		g.InputProtocol = strings.ToLower(newConf.NtripInputProtocol)
	} else {
		g.InputProtocol = newConf.CorrectionSource
	}

	nmeaConf := &gpsnmea.Config{
		ConnectionType: newConf.ConnectionType,
		DisableNMEA:    false,
	}

	// Init NMEAMovementSensor
	switch strings.ToLower(newConf.ConnectionType) {
	case serialStr:
		var err error
		nmeaConf.SerialConfig = (*gpsnmea.SerialConfig)(newConf.SerialConfig)
		g.Nmeamovementsensor, err = gpsnmea.NewSerialGPSNMEA(ctx, conf.ResourceName(), nmeaConf, logger)
		if err != nil {
			return nil, err
		}
	case i2cStr:
		var err error
		nmeaConf.Board = newConf.I2CConfig.Board
		nmeaConf.I2CConfig = &gpsnmea.I2CConfig{I2CBus: newConf.I2CBus, I2CBaudRate: newConf.I2CBaudRate, I2cAddr: newConf.I2cAddr}
		g.Nmeamovementsensor, err = gpsnmea.NewPmtkI2CGPSNMEA(ctx, deps, conf.ResourceName(), nmeaConf, logger)
		if err != nil {
			return nil, err
		}
	default:
		// Invalid protocol
		return nil, fmt.Errorf("%s is not a valid connection type", newConf.ConnectionType)
	}

	// Init ntripInfo from attributes
	g.ntripClient, err = NewNtripInfo(newConf.NtripConfig, g.logger)
	if err != nil {
		return nil, err
	}

	// baud rate
	if newConf.NtripBaud == 0 {
		newConf.NtripBaud = 38400
		g.logger.Info("ntrip_baud using default baud rate 38400")
	}
	g.Wbaud = newConf.NtripBaud

	switch g.InputProtocol {
	case serialStr:
		switch newConf.NtripPath {
		case "":
			g.logger.Info("RTK will use the same serial path as the GPS data to write RCTM messages")
			g.Writepath = newConf.SerialPath
		default:
			g.Writepath = newConf.NtripPath
		}
	case i2cStr:
		g.Addr = byte(newConf.I2cAddr)

		b, err := board.FromDependencies(deps, newConf.Board)
		if err != nil {
			return nil, fmt.Errorf("gps init: failed to find board: %w", err)
		}
		localB, ok := b.(board.LocalBoard)
		if !ok {
			return nil, fmt.Errorf("board %s is not local", newConf.Board)
		}

		i2cbus, ok := localB.I2CByName(newConf.I2CBus)
		if !ok {
			return nil, fmt.Errorf("gps init: failed to find i2c bus %s", newConf.I2CBus)
		}
		g.Bus = i2cbus
	}

	if err := g.start(); err != nil {
		return nil, err
	}
	return g, g.err.Get()
}

// Start begins NTRIP receiver with specified protocol and begins reading/updating MovementSensor measurements.
func (g *RTKMovementSensor) start() error {
	// TODO(RDK-1639): Test out what happens if we call this line and then the ReceiveAndWrite*
	// correction data goes wrong. Could anything worse than uncorrected data occur?
	if err := g.Nmeamovementsensor.Start(g.cancelCtx); err != nil {
		g.lastposition.GetLastPosition()
		return err
	}

	switch g.InputProtocol {
	case serialStr:
		g.activeBackgroundWorkers.Add(1)
		utils.PanicCapturingGo(g.receiveAndWriteSerial)
	case i2cStr:
		g.activeBackgroundWorkers.Add(1)
		utils.PanicCapturingGo(func() { g.receiveAndWriteI2C(g.cancelCtx) })
	}

	return g.err.Get()
}

// Connect attempts to connect to ntrip client until successful connection or timeout.
func (g *RTKMovementSensor) Connect(casterAddr, user, pwd string, maxAttempts int) error {
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

	g.logger.Debug("Connected to NTRIP caster")
	g.ntripMu.Lock()
	g.ntripClient.Client = c
	g.ntripMu.Unlock()
	return g.err.Get()
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

// receiveAndWriteI2C connects to NTRIP receiver and sends correction stream to the MovementSensor through I2C protocol.
func (g *RTKMovementSensor) receiveAndWriteI2C(ctx context.Context) {
	defer g.activeBackgroundWorkers.Done()
	if err := g.cancelCtx.Err(); err != nil {
		return
	}
	err := g.Connect(g.ntripClient.URL, g.ntripClient.Username, g.ntripClient.Password, g.ntripClient.MaxConnectAttempts)
	if err != nil {
		g.err.Set(err)
		return
	}

	if !g.ntripClient.Client.IsCasterAlive() {
		g.logger.Infof("caster %s seems to be down", g.ntripClient.URL)
	}

	// establish I2C connection
	handle, err := g.Bus.OpenHandle(g.Addr)
	if err != nil {
		g.logger.Errorf("can't open gps i2c %s", err)
		g.err.Set(err)
		return
	}
	// Send GLL, RMC, VTG, GGA, GSA, and GSV sentences each 1000ms
	baudcmd := fmt.Sprintf("PMTK251,%d", g.Wbaud)
	cmd251 := addChk([]byte(baudcmd))
	cmd314 := addChk([]byte("PMTK314,1,1,1,1,1,0,0,0,0,0,0,0,0,0,0,0,0,0,0"))
	cmd220 := addChk([]byte("PMTK220,1000"))

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

	err = g.GetStream(g.ntripClient.MountPoint, g.ntripClient.MaxConnectAttempts)
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
	wI2C := addChk(buf[:n])

	// port still open
	err = handle.Write(ctx, wI2C)
	if err != nil {
		g.logger.Errorf("i2c handle write failed %s", err)
		g.err.Set(err)
		return
	}

	scanner := rtcm3.NewScanner(r)

	g.ntripMu.Lock()
	g.ntripStatus = true
	g.ntripMu.Unlock()

	// It's okay to skip the mutex on this next line: g.ntripStatus can only be mutated by this
	// goroutine itself.
	for g.ntripStatus {
		select {
		case <-g.cancelCtx.Done():
			g.err.Set(err)
			return
		default:
		}

		// establish I2C connection
		handle, err := g.Bus.OpenHandle(g.Addr)
		if err != nil {
			g.logger.Errorf("can't open gps i2c %s", err)
			g.err.Set(err)
			return
		}

		msg, err := scanner.NextMessage()
		if err != nil {
			g.ntripMu.Lock()
			g.ntripStatus = false
			g.ntripMu.Unlock()

			if msg == nil {
				g.logger.Debug("No message... reconnecting to stream...")
				err = g.GetStream(g.ntripClient.MountPoint, g.ntripClient.MaxConnectAttempts)
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
				wI2C := addChk(buf[:n])

				err = handle.Write(ctx, wI2C)

				if err != nil {
					g.logger.Errorf("i2c handle write failed %s", err)
					g.err.Set(err)
					return
				}

				scanner = rtcm3.NewScanner(r)
				g.ntripMu.Lock()
				g.ntripStatus = true
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

// receiveAndWriteSerial connects to NTRIP receiver and sends correction stream to the MovementSensor through serial.
func (g *RTKMovementSensor) receiveAndWriteSerial() {
	defer g.activeBackgroundWorkers.Done()
	if err := g.cancelCtx.Err(); err != nil {
		return
	}
	err := g.Connect(g.ntripClient.URL, g.ntripClient.Username, g.ntripClient.Password, g.ntripClient.MaxConnectAttempts)
	if err != nil {
		g.err.Set(err)
		return
	}

	if !g.ntripClient.Client.IsCasterAlive() {
		g.logger.Infof("caster %s seems to be down", g.ntripClient.URL)
	}

	options := slib.OpenOptions{
		PortName:        g.Writepath,
		BaudRate:        uint(g.Wbaud),
		DataBits:        8,
		StopBits:        1,
		MinimumReadSize: 1,
	}

	// Open the port.
	g.ntripMu.Lock()
	if err := g.cancelCtx.Err(); err != nil {
		g.ntripMu.Unlock()
		return
	}
	g.CorrectionWriter, err = slib.Open(options)
	g.ntripMu.Unlock()
	if err != nil {
		g.logger.Errorf("serial.Open: %v", err)
		g.err.Set(err)
		return
	}

	w := bufio.NewWriter(g.CorrectionWriter)

	err = g.GetStream(g.ntripClient.MountPoint, g.ntripClient.MaxConnectAttempts)
	if err != nil {
		g.err.Set(err)
		return
	}

	r := io.TeeReader(g.ntripClient.Stream, w)
	scanner := rtcm3.NewScanner(r)

	g.ntripMu.Lock()
	g.ntripStatus = true
	g.ntripMu.Unlock()

	// It's okay to skip the mutex on this next line: g.ntripStatus can only be mutated by this
	// goroutine itself.
	for g.ntripStatus {
		select {
		case <-g.cancelCtx.Done():
			return
		default:
		}

		msg, err := scanner.NextMessage()
		if err != nil {
			g.ntripMu.Lock()
			g.ntripStatus = false
			g.ntripMu.Unlock()

			if msg == nil {
				g.logger.Debug("No message... reconnecting to stream...")
				err = g.GetStream(g.ntripClient.MountPoint, g.ntripClient.MaxConnectAttempts)
				if err != nil {
					g.err.Set(err)
					return
				}

				r = io.TeeReader(g.ntripClient.Stream, w)
				scanner = rtcm3.NewScanner(r)
				g.ntripMu.Lock()
				g.ntripStatus = true
				g.ntripMu.Unlock()
				continue
			}
		}
	}
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
	readings, err := movementsensor.Readings(ctx, g, extra)
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

// TODO: move these to utils
// PMTK checksums commands by XORing together each byte.
func addChk(data []byte) []byte {
	chk := checksum(data)
	newCmd := []byte("$")
	newCmd = append(newCmd, data...)
	newCmd = append(newCmd, []byte("*")...)
	newCmd = append(newCmd, chk)
	return newCmd
}

func checksum(data []byte) byte {
	var chk byte
	for _, b := range data {
		chk ^= b
	}
	return chk
}
