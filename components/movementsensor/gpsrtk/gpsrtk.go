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
	"sync"

	"github.com/de-bkg/gognss/pkg/ntrip"
	"github.com/edaniels/golog"
	"github.com/go-gnss/rtcm/rtcm3"
	"github.com/golang/geo/r3"
	slib "github.com/jacobsa/go-serial/serial"
	geo "github.com/kellydunn/golang-geo"
	"go.viam.com/utils"

	"go.viam.com/rdk/components/board"
	"go.viam.com/rdk/components/generic"
	"go.viam.com/rdk/components/movementsensor"
	gpsnmea "go.viam.com/rdk/components/movementsensor/gpsnmea"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/spatialmath"
	rdkutils "go.viam.com/rdk/utils"
)

// AttrConfig is used for converting NMEA MovementSensor with RTK capabilities config attributes.
type AttrConfig struct {
	CorrectionSource string `json:"correction_source"`
	Board            string `json:"board,omitempty"`
	ConnectionType   string `json:"connection_type,omitempty"`

	*SerialAttrConfig `json:"serial_attributes,omitempty"`
	*I2CAttrConfig    `json:"i2c_attributes,omitempty"`
	*NtripAttrConfig  `json:"ntrip_attributes,omitempty"`
}

// NtripAttrConfig is used for converting attributes for a correction source.
type NtripAttrConfig struct {
	NtripAddr            string `json:"ntrip_addr"`
	NtripConnectAttempts int    `json:"ntrip_connect_attempts,omitempty"`
	NtripMountpoint      string `json:"ntrip_mountpoint,omitempty"`
	NtripPass            string `json:"ntrip_password,omitempty"`
	NtripUser            string `json:"ntrip_username,omitempty"`
	NtripPath            string `json:"ntrip_path,omitempty"`
	NtripBaud            int    `json:"ntrip_baud,omitempty"`
	NtripInputProtocol   string `json:"ntrip_input_protocol,omitempty"`
}

// SerialAttrConfig is used for converting attributes for a correction source.
type SerialAttrConfig struct {
	SerialPath               string `json:"serial_path"`
	SerialBaudRate           int    `json:"serial_baud_rate,omitempty"`
	SerialCorrectionPath     string `json:"serial_correction_path,omitempty"`
	SerialCorrectionBaudRate int    `json:"serial_correction_baud_rate,omitempty"`
}

// I2CAttrConfig is used for converting attributes for a correction source.
type I2CAttrConfig struct {
	I2CBus      string `json:"i2c_bus"`
	I2cAddr     int    `json:"i2c_addr"`
	I2CBaudRate int    `json:"i2c_baud_rate,omitempty"`
}

// Validate ensures all parts of the config are valid.
func (cfg *AttrConfig) Validate(path string) ([]string, error) {
	var deps []string
	switch cfg.CorrectionSource {
	case ntripStr:
		return nil, cfg.NtripAttrConfig.ValidateNtrip(path)
	case i2cStr:
		if cfg.Board == "" {
			return nil, utils.NewConfigValidationFieldRequiredError(path, "board")
		}
		deps = append(deps, cfg.Board)
		return deps, cfg.I2CAttrConfig.ValidateI2C(path)
	case serialStr:
		return nil, cfg.SerialAttrConfig.ValidateSerial(path)
	case "":
		return nil, utils.NewConfigValidationFieldRequiredError(path, "correction_source")
	default:
		return nil, utils.NewConfigValidationFieldRequiredError(path, "correction_source")
	}
}

// ValidateI2C ensures all parts of the config are valid.
func (cfg *I2CAttrConfig) ValidateI2C(path string) error {
	if cfg.I2CBus == "" {
		return utils.NewConfigValidationFieldRequiredError(path, "i2c_bus")
	}
	if cfg.I2cAddr == 0 {
		return utils.NewConfigValidationFieldRequiredError(path, "i2c_addr")
	}

	return nil
}

// ValidateSerial ensures all parts of the config are valid.
func (cfg *SerialAttrConfig) ValidateSerial(path string) error {
	if cfg.SerialPath == "" {
		return utils.NewConfigValidationFieldRequiredError(path, "serial_path")
	}
	return nil
}

// ValidateNtrip ensures all parts of the config are valid.
func (cfg *NtripAttrConfig) ValidateNtrip(path string) error {
	if cfg.NtripAddr == "" {
		return utils.NewConfigValidationFieldRequiredError(path, "ntrip_addr")
	}
	if cfg.NtripPath == "" {
		return utils.NewConfigValidationFieldRequiredError(path, "ntrip_path")
	}
	return nil
}

const roverModel = "gps-rtk"

func init() {
	registry.RegisterComponent(
		movementsensor.Subtype,
		roverModel,
		registry.Component{Constructor: func(
			ctx context.Context,
			deps registry.Dependencies,
			cfg config.Component,
			logger golog.Logger,
		) (interface{}, error) {
			return newRTKStation(ctx, deps, cfg, logger)
		}})

	config.RegisterComponentAttributeMapConverter(movementsensor.SubtypeName, roverModel,
		func(attributes config.AttributeMap) (interface{}, error) {
			var attr StationConfig
			return config.TransformAttributeMapToStruct(&attr, attributes)
		},
		&StationConfig{})
}

// A RTKMovementSensor is an NMEA MovementSensor model that can intake RTK correction data.
type RTKMovementSensor struct {
	generic.Unimplemented
	logger     golog.Logger
	cancelCtx  context.Context
	cancelFunc func()

	activeBackgroundWorkers sync.WaitGroup
	errMu                   sync.Mutex
	lastError               error

	nmeamovementsensor gpsnmea.NmeaMovementSensor
	inputProtocol      string
	ntripClient        *NtripInfo
	correctionWriter   io.ReadWriteCloser
	ntripStatus        bool

	bus       board.I2C
	wbaud     int
	addr      byte // for i2c only
	writepath string
}

func newRTKMovementSensor(
	ctx context.Context,
	deps registry.Dependencies,
	cfg config.Component,
	logger golog.Logger,
) (movementsensor.MovementSensor, error) {
	attr, ok := cfg.ConvertedAttributes.(*AttrConfig)
	if !ok {
		return nil, rdkutils.NewUnexpectedTypeError(attr, cfg.ConvertedAttributes)
	}

	logger.Debug("Returning n")

	cancelCtx, cancelFunc := context.WithCancel(ctx)

	g := &RTKMovementSensor{
		cancelCtx:  cancelCtx,
		cancelFunc: cancelFunc,
		logger:     logger,
	}

	g.inputProtocol = attr.CorrectionSource

	nmeaAttr := &gpsnmea.AttrConfig{
		ConnectionType: attr.ConnectionType,
		Board:          attr.Board,
		DisableNMEA:    false,
	}

	// Init NMEAMovementSensor
	switch g.inputProtocol {
	case serialStr:
		var err error
		nmeaAttr.SerialAttrConfig = (*gpsnmea.SerialAttrConfig)(attr.SerialAttrConfig)
		g.nmeamovementsensor, err = gpsnmea.NewSerialGPSNMEA(ctx, nmeaAttr, logger)
		if err != nil {
			return nil, err
		}
	case i2cStr:
		var err error
		nmeaAttr.I2CAttrConfig = (*gpsnmea.I2CAttrConfig)(attr.I2CAttrConfig)
		g.nmeamovementsensor, err = gpsnmea.NewPmtkI2CGPSNMEA(ctx, deps, nmeaAttr, logger)
		if err != nil {
			return nil, err
		}
	default:
		// Invalid protocol
		return nil, fmt.Errorf("%s is not a valid protocol", g.inputProtocol)
	}

	// Init ntripInfo from attributes

	g.ntripClient = &NtripInfo{
		URL:                attr.NtripAddr,
		Username:           attr.NtripUser,
		Password:           attr.NtripPass,
		MountPoint:         attr.NtripMountpoint,
		Client:             &ntrip.Client{},
		Stream:             nil,
		MaxConnectAttempts: 0,
	}

	// baud rate
	g.wbaud = attr.NtripBaud
	if g.wbaud == 38400 {
		g.logger.Info("ntrip_baud using default baud rate 38400")
	}

	if g.writepath != "" {
		g.logger.Info("ntrip_path will use same path for writing RCTM messages to gps")
		g.writepath = attr.NtripPath
	}

	// I2C address only, assumes address is correct since this was checked when gps was initialized
	g.addr = byte(attr.I2cAddr)

	if err := g.Start(ctx); err != nil {
		return nil, err
	}
	return g, g.lastError
}

func (g *RTKMovementSensor) setLastError(err error) {
	g.errMu.Lock()
	defer g.errMu.Unlock()

	g.lastError = err
}

// Start begins NTRIP receiver with specified protocol and begins reading/updating MovementSensor measurements.
func (g *RTKMovementSensor) Start(ctx context.Context) error {
	switch g.inputProtocol {
	case serialStr:
		go g.ReceiveAndWriteSerial()
	case i2cStr:
		go g.ReceiveAndWriteI2C(ctx)
	}
	if err := g.nmeamovementsensor.Start(ctx); err != nil {
		return err
	}

	return g.lastError
}

// Connect attempts to connect to ntrip client until successful connection or timeout.
func (g *RTKMovementSensor) Connect(casterAddr, user, pwd string, maxAttempts int) error {
	success := false
	attempts := 0

	var c *ntrip.Client
	var err error

	g.logger.Debug("Connecting to NTRIP caster")
	for !success && attempts < maxAttempts {
		select {
		case <-g.cancelCtx.Done():
			return errors.New("Canceled")
		default:
		}

		c, err = ntrip.NewClient(casterAddr, ntrip.Options{Username: user, Password: pwd})
		if err == nil {
			success = true
		}

		attempts++
	}

	if err != nil {
		g.logger.Errorf("Can't connect to NTRIP caster: %s", err)
		return err
	}
	g.ntripClient.Client = c
	g.logger.Debug("Connected to NTRIP caster")

	return g.lastError
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

		rc, err = g.ntripClient.Client.GetStream(mountPoint)
		if err == nil {
			success = true
		}
		attempts++
	}

	if err != nil {
		g.logger.Errorf("Can't connect to NTRIP stream: %s", err)
		return err
	}

	g.ntripClient.Stream = rc

	g.logger.Debug("Connected to stream")

	return g.lastError
}

// ReceiveAndWriteI2C connects to NTRIP receiver and sends correction stream to the MovementSensor through I2C protocol.
func (g *RTKMovementSensor) ReceiveAndWriteI2C(ctx context.Context) {
	g.activeBackgroundWorkers.Add(1)
	defer g.activeBackgroundWorkers.Done()
	err := g.Connect(g.ntripClient.URL, g.ntripClient.Username, g.ntripClient.Password, g.ntripClient.MaxConnectAttempts)
	if err != nil {
		g.setLastError(err)
		return
	}

	if !g.ntripClient.Client.IsCasterAlive() {
		g.logger.Infof("caster %s seems to be down", g.ntripClient.URL)
	}

	// establish I2C connection
	handle, err := g.bus.OpenHandle(g.addr)
	if err != nil {
		g.logger.Errorf("can't open gps i2c %s", err)
		g.setLastError(err)
		return
	}
	// Send GLL, RMC, VTG, GGA, GSA, and GSV sentences each 1000ms
	baudcmd := fmt.Sprintf("PMTK251,%d", g.wbaud)
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
		g.setLastError(err)
		return
	}
	err = handle.Write(ctx, cmd220)
	if err != nil {
		g.logger.Debug("failed to set NMEA update rate")
		g.setLastError(err)
		return
	}

	err = g.GetStream(g.ntripClient.MountPoint, g.ntripClient.MaxConnectAttempts)
	if err != nil {
		g.setLastError(err)
		return
	}

	// create a buffer
	w := &bytes.Buffer{}
	r := io.TeeReader(g.ntripClient.Stream, w)

	buf := make([]byte, 1100)
	n, err := g.ntripClient.Stream.Read(buf)
	if err != nil {
		g.setLastError(err)
		return
	}
	wI2C := addChk(buf[:n])

	// port still open
	err = handle.Write(ctx, wI2C)
	if err != nil {
		g.logger.Errorf("i2c handle write failed %s", err)
		g.setLastError(err)
		return
	}

	scanner := rtcm3.NewScanner(r)

	g.ntripStatus = true

	for g.ntripStatus {
		select {
		case <-g.cancelCtx.Done():
			g.setLastError(err)
			return
		default:
		}

		// establish I2C connection
		handle, err := g.bus.OpenHandle(g.addr)
		if err != nil {
			g.logger.Errorf("can't open gps i2c %s", err)
			g.setLastError(err)
			return
		}

		msg, err := scanner.NextMessage()
		if err != nil {
			g.ntripStatus = false
			if msg == nil {
				g.logger.Debug("No message... reconnecting to stream...")
				err = g.GetStream(g.ntripClient.MountPoint, g.ntripClient.MaxConnectAttempts)
				if err != nil {
					g.setLastError(err)
					return
				}

				w = &bytes.Buffer{}
				r = io.TeeReader(g.ntripClient.Stream, w)

				buf = make([]byte, 1100)
				n, err := g.ntripClient.Stream.Read(buf)
				if err != nil {
					g.setLastError(err)
					return
				}
				wI2C := addChk(buf[:n])

				err = handle.Write(ctx, wI2C)

				if err != nil {
					g.logger.Errorf("i2c handle write failed %s", err)
					g.setLastError(err)
					return
				}

				scanner = rtcm3.NewScanner(r)
				g.ntripStatus = true
				continue
			}
		}
		// close I2C
		err = handle.Close()
		if err != nil {
			g.logger.Debug("failed to close handle: %s", err)
			g.setLastError(err)
			return
		}
	}
}

// ReceiveAndWriteSerial connects to NTRIP receiver and sends correction stream to the MovementSensor through serial.
func (g *RTKMovementSensor) ReceiveAndWriteSerial() {
	g.activeBackgroundWorkers.Add(1)
	defer g.activeBackgroundWorkers.Done()
	err := g.Connect(g.ntripClient.URL, g.ntripClient.Username, g.ntripClient.Password, g.ntripClient.MaxConnectAttempts)
	if err != nil {
		g.setLastError(err)
		return
	}

	if !g.ntripClient.Client.IsCasterAlive() {
		g.logger.Infof("caster %s seems to be down", g.ntripClient.URL)
	}

	options := slib.OpenOptions{
		PortName:        g.writepath,
		BaudRate:        uint(g.wbaud),
		DataBits:        8,
		StopBits:        1,
		MinimumReadSize: 1,
	}

	// Open the port.
	g.correctionWriter, err = slib.Open(options)
	if err != nil {
		g.logger.Errorf("serial.Open: %v", err)
		g.setLastError(err)
		return
	}

	w := bufio.NewWriter(g.correctionWriter)

	err = g.GetStream(g.ntripClient.MountPoint, g.ntripClient.MaxConnectAttempts)
	if err != nil {
		g.setLastError(err)
		return
	}

	r := io.TeeReader(g.ntripClient.Stream, w)
	scanner := rtcm3.NewScanner(r)

	g.ntripStatus = true

	for g.ntripStatus {
		select {
		case <-g.cancelCtx.Done():
			return
		default:
		}

		msg, err := scanner.NextMessage()
		if err != nil {
			g.ntripStatus = false
			if msg == nil {
				g.logger.Debug("No message... reconnecting to stream...")
				err = g.GetStream(g.ntripClient.MountPoint, g.ntripClient.MaxConnectAttempts)
				if err != nil {
					g.setLastError(err)
					return
				}

				r = io.TeeReader(g.ntripClient.Stream, w)
				scanner = rtcm3.NewScanner(r)
				g.ntripStatus = true
				continue
			}
		}
	}
}

// NtripStatus returns true if connection to NTRIP stream is OK, false if not.
func (g *RTKMovementSensor) NtripStatus() (bool, error) {
	return g.ntripStatus, g.lastError
}

// Position returns the current geographic location of the MOVEMENTSENSOR.
func (g *RTKMovementSensor) Position(ctx context.Context) (*geo.Point, float64, error) {
	if g.lastError != nil {
		return &geo.Point{}, 0, g.lastError
	}
	return g.nmeamovementsensor.Position(ctx)
}

// LinearVelocity passthrough.
func (g *RTKMovementSensor) LinearVelocity(ctx context.Context) (r3.Vector, error) {
	if g.lastError != nil {
		return r3.Vector{}, g.lastError
	}
	return g.nmeamovementsensor.LinearVelocity(ctx)
}

// AngularVelocity passthrough.
func (g *RTKMovementSensor) AngularVelocity(ctx context.Context) (spatialmath.AngularVelocity, error) {
	if g.lastError != nil {
		return spatialmath.AngularVelocity{}, g.lastError
	}
	return g.nmeamovementsensor.AngularVelocity(ctx)
}

// CompassHeading passthrough.
func (g *RTKMovementSensor) CompassHeading(ctx context.Context) (float64, error) {
	if g.lastError != nil {
		return 0, g.lastError
	}
	return g.nmeamovementsensor.CompassHeading(ctx)
}

// Orientation passthrough.
func (g *RTKMovementSensor) Orientation(ctx context.Context) (spatialmath.Orientation, error) {
	if g.lastError != nil {
		return spatialmath.NewZeroOrientation(), g.lastError
	}
	return g.nmeamovementsensor.Orientation(ctx)
}

// ReadFix passthrough.
func (g *RTKMovementSensor) ReadFix(ctx context.Context) (int, error) {
	if g.lastError != nil {
		return 0, g.lastError
	}
	return g.nmeamovementsensor.ReadFix(ctx)
}

// Properties passthrough.
func (g *RTKMovementSensor) Properties(ctx context.Context) (*movementsensor.Properties, error) {
	if g.lastError != nil {
		return &movementsensor.Properties{}, g.lastError
	}
	return g.nmeamovementsensor.Properties(ctx)
}

// Accuracy passthrough.
func (g *RTKMovementSensor) Accuracy(ctx context.Context) (map[string]float32, error) {
	if g.lastError != nil {
		return map[string]float32{}, g.lastError
	}
	return g.nmeamovementsensor.Accuracy(ctx)
}

// Readings will use the default MovementSensor Readings if not provided.
func (g *RTKMovementSensor) Readings(ctx context.Context, extra map[string]interface{}) (map[string]interface{}, error) {
	readings, err := movementsensor.Readings(ctx, g)
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
func (g *RTKMovementSensor) Close() error {
	g.logger.Debug("closing rtk gps")
	g.cancelFunc()
	g.activeBackgroundWorkers.Wait()

	err := g.nmeamovementsensor.Close()
	if err != nil {
		return err
	}

	// close ntrip writer
	if g.correctionWriter != nil {
		if err := g.correctionWriter.Close(); err != nil {
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
			return err
		}
		g.ntripClient.Stream = nil
	}

	g.logger.Debug("closed rtk gps")
	return g.lastError
}
