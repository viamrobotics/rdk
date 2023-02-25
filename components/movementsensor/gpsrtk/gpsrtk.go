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
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/spatialmath"
	rdkutils "go.viam.com/rdk/utils"
)

// ErrRoverValidation contains the model substring for the available correction source types.
var ErrRoverValidation = fmt.Errorf("only serial, I2C, and ntrip are supported correction sources for %s", roverModel.Name)

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
		return nil, ErrRoverValidation
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

var roverModel = resource.NewDefaultModel("gps-rtk")

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
			return newRTKMovementSensor(ctx, deps, cfg, logger)
		}})

	config.RegisterComponentAttributeMapConverter(movementsensor.Subtype, roverModel,
		func(attributes config.AttributeMap) (interface{}, error) {
			var attr AttrConfig
			return config.TransformAttributeMapToStruct(&attr, attributes)
		},
		&AttrConfig{})
}

// A RTKMovementSensor is an NMEA MovementSensor model that can intake RTK correction data.
type RTKMovementSensor struct {
	generic.Unimplemented
	logger     golog.Logger
	cancelCtx  context.Context
	cancelFunc func()

	activeBackgroundWorkers sync.WaitGroup
	// Lock the mutex whenever you interact with ntripClient or ntripStatus.
	mu sync.Mutex

	err movementsensor.LastError

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

	if err := g.Start(); err != nil {
		return nil, err
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	return g, g.err.Get()
}

// Start begins NTRIP receiver with specified protocol and begins reading/updating MovementSensor measurements.
func (g *RTKMovementSensor) Start() error {
	// TODO(RDK-1639): Test out what happens if we call this line and then the ReceiveAndWrite*
	// correction data goes wrong. Could anything worse than uncorrected data occur?
	if err := g.nmeamovementsensor.Start(g.cancelCtx); err != nil {
		return err
	}

	switch g.inputProtocol {
	case serialStr:
		utils.PanicCapturingGo(g.ReceiveAndWriteSerial)
	case i2cStr:
		utils.PanicCapturingGo(func() { g.ReceiveAndWriteI2C(g.cancelCtx) })
	}

	return g.err.Get()
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

	g.logger.Debug("Connected to NTRIP caster")
	g.mu.Lock()
	defer g.mu.Unlock()

	g.ntripClient.Client = c
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
		g.logger.Errorf("Can't connect to NTRIP stream: %s", err)
		return err
	}

	g.logger.Debug("Connected to stream")
	g.mu.Lock()
	defer g.mu.Unlock()

	g.ntripClient.Stream = rc
	return g.err.Get()
}

// ReceiveAndWriteI2C connects to NTRIP receiver and sends correction stream to the MovementSensor through I2C protocol.
func (g *RTKMovementSensor) ReceiveAndWriteI2C(ctx context.Context) {
	g.activeBackgroundWorkers.Add(1)
	defer g.activeBackgroundWorkers.Done()
	err := g.Connect(g.ntripClient.URL, g.ntripClient.Username, g.ntripClient.Password, g.ntripClient.MaxConnectAttempts)
	if err != nil {
		g.err.Set(err)
		return
	}

	if !g.ntripClient.Client.IsCasterAlive() {
		g.logger.Infof("caster %s seems to be down", g.ntripClient.URL)
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

	g.mu.Lock()
	g.ntripStatus = true
	g.mu.Unlock()

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
		handle, err := g.bus.OpenHandle(g.addr)
		if err != nil {
			g.logger.Errorf("can't open gps i2c %s", err)
			g.err.Set(err)
			return
		}

		msg, err := scanner.NextMessage()
		if err != nil {
			g.mu.Lock()
			g.ntripStatus = false
			g.mu.Unlock()

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
				g.mu.Lock()
				g.ntripStatus = true
				g.mu.Unlock()
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

// ReceiveAndWriteSerial connects to NTRIP receiver and sends correction stream to the MovementSensor through serial.
func (g *RTKMovementSensor) ReceiveAndWriteSerial() {
	g.activeBackgroundWorkers.Add(1)
	defer g.activeBackgroundWorkers.Done()
	err := g.Connect(g.ntripClient.URL, g.ntripClient.Username, g.ntripClient.Password, g.ntripClient.MaxConnectAttempts)
	if err != nil {
		g.err.Set(err)
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
		g.err.Set(err)
		return
	}

	w := bufio.NewWriter(g.correctionWriter)

	err = g.GetStream(g.ntripClient.MountPoint, g.ntripClient.MaxConnectAttempts)
	if err != nil {
		g.err.Set(err)
		return
	}

	r := io.TeeReader(g.ntripClient.Stream, w)
	scanner := rtcm3.NewScanner(r)

	g.mu.Lock()
	g.ntripStatus = true
	g.mu.Unlock()

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
			g.mu.Lock()
			g.ntripStatus = false
			g.mu.Unlock()

			if msg == nil {
				g.logger.Debug("No message... reconnecting to stream...")
				err = g.GetStream(g.ntripClient.MountPoint, g.ntripClient.MaxConnectAttempts)
				if err != nil {
					g.err.Set(err)
					return
				}

				r = io.TeeReader(g.ntripClient.Stream, w)
				scanner = rtcm3.NewScanner(r)
				g.mu.Lock()
				g.ntripStatus = true
				g.mu.Unlock()
				continue
			}
		}
	}
}

// NtripStatus returns true if connection to NTRIP stream is OK, false if not.
func (g *RTKMovementSensor) NtripStatus() (bool, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.ntripStatus, g.err.Get()
}

// Position returns the current geographic location of the MOVEMENTSENSOR.
func (g *RTKMovementSensor) Position(ctx context.Context, extra map[string]interface{}) (*geo.Point, float64, error) {
	g.mu.Lock()
	lastError := g.err.Get()
	if lastError != nil {
		defer g.mu.Unlock()
		return geo.NewPoint(0, 0), 0, lastError
	}
	g.mu.Unlock()

	return g.nmeamovementsensor.Position(ctx, extra)
}

// LinearVelocity passthrough.
func (g *RTKMovementSensor) LinearVelocity(ctx context.Context, extra map[string]interface{}) (r3.Vector, error) {
	g.mu.Lock()
	lastError := g.err.Get()
	if lastError != nil {
		defer g.mu.Unlock()
		return r3.Vector{}, lastError
	}
	g.mu.Unlock()

	return g.nmeamovementsensor.LinearVelocity(ctx, extra)
}

// LinearAcceleration passthrough.
func (g *RTKMovementSensor) LinearAcceleration(ctx context.Context, extra map[string]interface{}) (r3.Vector, error) {
	lastError := g.err.Get()
	if lastError != nil {
		return r3.Vector{}, lastError
	}
	return g.nmeamovementsensor.LinearAcceleration(ctx, extra)
}

// AngularVelocity passthrough.
func (g *RTKMovementSensor) AngularVelocity(ctx context.Context, extra map[string]interface{}) (spatialmath.AngularVelocity, error) {
	g.mu.Lock()
	lastError := g.err.Get()
	if lastError != nil {
		defer g.mu.Unlock()
		return spatialmath.AngularVelocity{}, lastError
	}
	g.mu.Unlock()

	return g.nmeamovementsensor.AngularVelocity(ctx, extra)
}

// CompassHeading passthrough.
func (g *RTKMovementSensor) CompassHeading(ctx context.Context, extra map[string]interface{}) (float64, error) {
	g.mu.Lock()
	lastError := g.err.Get()
	if lastError != nil {
		defer g.mu.Unlock()
		return 0, lastError
	}
	g.mu.Unlock()

	return g.nmeamovementsensor.CompassHeading(ctx, extra)
}

// Orientation passthrough.
func (g *RTKMovementSensor) Orientation(ctx context.Context, extra map[string]interface{}) (spatialmath.Orientation, error) {
	g.mu.Lock()
	lastError := g.err.Get()
	if lastError != nil {
		defer g.mu.Unlock()
		return spatialmath.NewZeroOrientation(), lastError
	}
	g.mu.Unlock()

	return g.nmeamovementsensor.Orientation(ctx, extra)
}

// ReadFix passthrough.
func (g *RTKMovementSensor) ReadFix(ctx context.Context) (int, error) {
	g.mu.Lock()
	lastError := g.err.Get()
	if lastError != nil {
		defer g.mu.Unlock()
		return 0, lastError
	}
	g.mu.Unlock()

	return g.nmeamovementsensor.ReadFix(ctx)
}

// Properties passthrough.
func (g *RTKMovementSensor) Properties(ctx context.Context, extra map[string]interface{}) (*movementsensor.Properties, error) {
	g.mu.Lock()
	lastError := g.err.Get()
	if lastError != nil {
		defer g.mu.Unlock()
		return &movementsensor.Properties{}, lastError
	}
	g.mu.Unlock()

	return g.nmeamovementsensor.Properties(ctx, extra)
}

// Accuracy passthrough.
func (g *RTKMovementSensor) Accuracy(ctx context.Context, extra map[string]interface{}) (map[string]float32, error) {
	g.mu.Lock()
	lastError := g.err.Get()
	if lastError != nil {
		defer g.mu.Unlock()
		return map[string]float32{}, lastError
	}
	g.mu.Unlock()

	return g.nmeamovementsensor.Accuracy(ctx, extra)
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
	return g.err.Get()
}
