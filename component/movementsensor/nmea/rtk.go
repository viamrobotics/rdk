package nmea

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

	"go.viam.com/rdk/component/board"
	"go.viam.com/rdk/component/generic"
	"go.viam.com/rdk/component/movementsensor"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/spatialmath"
)

// RTKAttrConfig is used for converting Serial NMEA MovementSensor config attributes.
type RTKAttrConfig struct {
	// Serial
	SerialPath     string `json:"path"`
	CorrectionPath string `json:"correction_path"`

	// I2C
	Board   string `json:"board"`
	Bus     string `json:"bus"`
	I2cAddr int    `json:"i2c_addr"`

	// Ntrip
	NtripAddr            string `json:"ntrip_addr"`
	NtripConnectAttempts int    `json:"ntrip_connect_attempts,omitempty"`
	NtripMountpoint      string `json:"ntrip_mountpoint,omitempty"`
	NtripPass            string `json:"ntrip_password,omitempty"`
	NtripUser            string `json:"ntrip_username,omitempty"`
	NtripPath            string `json:"ntrip_path,omitempty"`
	NtripBaud            string `json:"ntrip_baud,omitempty"`
	NtripInputProtocol   string `json:"ntrip_input_protocol,omitempty"`
}

// ValidateRTK ensures all parts of the config are valid.
func (config *RTKAttrConfig) ValidateRTK(path string) error {
	if len(config.NtripAddr) == 0 {
		return errors.New("expected nonempty ntrip address")
	}

	if (len(config.NtripPath) == 0 && len(config.SerialPath) == 0) &&
		(len(config.Board) == 0 || len(config.Bus) == 0 || config.I2cAddr == 0) {
		return errors.New("expected either nonempty ntrip path, serial path, or I2C board, bus, and address")
	}

	return nil
}

func init() {
	registry.RegisterComponent(
		movementsensor.Subtype,
		"rtk",
		registry.Component{Constructor: func(
			ctx context.Context,
			deps registry.Dependencies,
			config config.Component,
			logger golog.Logger,
		) (interface{}, error) {
			return newRTKMovementSensor(ctx, deps, config, logger)
		}})
}

type nmeaMovementSensor interface {
	movementsensor.MovementSensor
	Start(ctx context.Context)                // Initialize and run MovementSensor
	Close() error                             // Close MovementSensor
	ReadFix(ctx context.Context) (int, error) // Returns the fix quality of the current MovementSensor measurements
}

// A RTKMovementSensor is an NMEA MovementSensor model that can intake RTK correction data.
type RTKMovementSensor struct {
	generic.Unimplemented
	nmeamovementsensor nmeaMovementSensor
	ntripInputProtocol string
	ntripClient        *NtripInfo
	logger             golog.Logger
	correctionWriter   io.ReadWriteCloser
	ntripStatus        bool

	bus       board.I2C
	wbaud     int
	addr      byte // for i2c only
	writepath string

	cancelCtx               context.Context
	cancelFunc              func()
	activeBackgroundWorkers sync.WaitGroup
}

// NtripInfo contains the information necessary to connect to a mountpoint.
type NtripInfo struct {
	URL                string
	Username           string
	Password           string
	MountPoint         string
	Client             *ntrip.Client
	Stream             io.ReadCloser
	MaxConnectAttempts int
}

const (
	ntripAddrAttrName          = "ntrip_addr"
	ntripUserAttrName          = "ntrip_username"
	ntripPassAttrName          = "ntrip_password"
	ntripMountPointAttrName    = "ntrip_mountpoint"
	ntripConnectAttemptsName   = "ntrip_connect_attempts"
	ntripPathAttrName          = "ntrip_path"
	ntripInputProtocolAttrName = "correction_input_protocol"
	baudAttrName               = "ntrip_baud"
)

// NewNtripInfo creates a new NtripInfo object given ntrip information in the configuration.
func NewNtripInfo(ctx context.Context, config config.Component, logger golog.Logger) (*NtripInfo, error) {
	n := &NtripInfo{}

	// Init NtripInfo from attributes
	n.URL = config.Attributes.String(ntripAddrAttrName)
	if n.URL == "" {
		return nil, fmt.Errorf("NTRIP expected non-empty string for %q", ntripAddrAttrName)
	}
	n.Username = config.Attributes.String(ntripUserAttrName)
	if n.Username == "" {
		logger.Info("ntrip_username set to empty")
	}
	n.Password = config.Attributes.String(ntripPassAttrName)
	if n.Password == "" {
		logger.Info("ntrip_password set to empty")
	}
	n.MountPoint = config.Attributes.String(ntripMountPointAttrName)
	if n.MountPoint == "" {
		logger.Info("ntrip_mountpoint set to empty")
	}
	n.MaxConnectAttempts = config.Attributes.Int(ntripConnectAttemptsName, 10)
	if n.MaxConnectAttempts == 10 {
		logger.Info("ntrip_connect_attempts using default 10")
	}

	logger.Debug("Returning n")
	return n, nil
}

func newRTKMovementSensor(
	ctx context.Context,
	deps registry.Dependencies,
	config config.Component,
	logger golog.Logger,
) (nmeaMovementSensor, error) {
	cancelCtx, cancelFunc := context.WithCancel(ctx)

	g := &RTKMovementSensor{cancelCtx: cancelCtx, cancelFunc: cancelFunc, logger: logger}

	g.ntripInputProtocol = config.Attributes.String(ntripInputProtocolAttrName)

	// Init NMEAMovementSensor
	switch g.ntripInputProtocol {
	case "serial":
		var err error
		g.nmeamovementsensor, err = newSerialNMEAMovementSensor(ctx, config, logger)
		if err != nil {
			return nil, err
		}
	case "I2C":
		var err error
		g.nmeamovementsensor, err = newPmtkI2CNMEAMovementSensor(ctx, deps, config, logger)
		if err != nil {
			return nil, err
		}
	default:
		// Invalid protocol
		return nil, fmt.Errorf("%s is not a valid protocol", g.ntripInputProtocol)
	}

	// Init ntripInfo from attributes
	ntripInfoComp, err := NewNtripInfo(ctx, config, logger)
	if err != nil {
		return nil, err
	}
	g.ntripClient = ntripInfoComp

	// baud rate
	g.wbaud = config.Attributes.Int(baudAttrName, 38400)
	if g.wbaud == 38400 {
		g.logger.Info("ntrip_baud using default baud rate 38400")
	}

	g.writepath = config.Attributes.String(ntripPathAttrName)
	if g.writepath == "" {
		g.logger.Info("ntrip_path will use same path for writing RCTM messages to gps")
		g.writepath = config.Attributes.String(pathAttrName)
	}

	// I2C address only, assumes address is correct since this was checked when gps was initialized
	g.addr = byte(config.Attributes.Int("i2c_addr", -1))

	g.Start(ctx)

	return g, nil
}

// Start begins NTRIP receiver with specified protocol and begins reading/updating MovementSensor measurements.
func (g *RTKMovementSensor) Start(ctx context.Context) {
	switch g.ntripInputProtocol {
	case "serial":
		go g.ReceiveAndWriteSerial()
	case "I2C":
		go g.ReceiveAndWriteI2C(ctx)
	}
	g.nmeamovementsensor.Start(ctx)
}

// Connect attempts to connect to ntrip client until successful connection or timeout.
func (g *RTKMovementSensor) Connect(casterAddr string, user string, pwd string, maxAttempts int) error {
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

	return nil
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

	return nil
}

// ReceiveAndWriteI2C connects to NTRIP receiver and sends correction stream to the MovementSensor through I2C protocol.
func (g *RTKMovementSensor) ReceiveAndWriteI2C(ctx context.Context) {
	g.activeBackgroundWorkers.Add(1)
	defer g.activeBackgroundWorkers.Done()
	err := g.Connect(g.ntripClient.URL, g.ntripClient.Username, g.ntripClient.Password, g.ntripClient.MaxConnectAttempts)
	if err != nil {
		return
	}

	if !g.ntripClient.Client.IsCasterAlive() {
		g.logger.Infof("caster %s seems to be down", g.ntripClient.URL)
	}

	// establish I2C connection
	handle, err := g.bus.OpenHandle(g.addr)
	if err != nil {
		g.logger.Fatalf("can't open gps i2c %s", err)
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
		return
	}
	err = handle.Write(ctx, cmd220)
	if err != nil {
		g.logger.Debug("failed to set NMEA update rate")
		return
	}

	err = g.GetStream(g.ntripClient.MountPoint, g.ntripClient.MaxConnectAttempts)
	if err != nil {
		return
	}

	// create a buffer
	w := &bytes.Buffer{}
	r := io.TeeReader(g.ntripClient.Stream, w)

	buf := make([]byte, 1100)
	n, err := g.ntripClient.Stream.Read(buf)
	if err != nil {
		return
	}
	wI2C := addChk(buf[:n])

	// port still open
	err = handle.Write(ctx, wI2C)
	if err != nil {
		g.logger.Fatalf("i2c handle write failed %s", err)
		return
	}

	scanner := rtcm3.NewScanner(r)

	g.ntripStatus = true

	for g.ntripStatus {
		select {
		case <-g.cancelCtx.Done():
			return
		default:
		}

		// establish I2C connection
		handle, err := g.bus.OpenHandle(g.addr)
		if err != nil {
			g.logger.Fatalf("can't open gps i2c %s", err)
			return
		}

		msg, err := scanner.NextMessage()
		if err != nil {
			g.ntripStatus = false
			if msg == nil {
				g.logger.Debug("No message... reconnecting to stream...")
				err = g.GetStream(g.ntripClient.MountPoint, g.ntripClient.MaxConnectAttempts)
				if err != nil {
					return
				}

				w = &bytes.Buffer{}
				r = io.TeeReader(g.ntripClient.Stream, w)

				buf = make([]byte, 1100)
				n, err := g.ntripClient.Stream.Read(buf)
				if err != nil {
					return
				}
				wI2C := addChk(buf[:n])

				err = handle.Write(ctx, wI2C)

				if err != nil {
					g.logger.Fatalf("i2c handle write failed %s", err)
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
		return
	}

	w := bufio.NewWriter(g.correctionWriter)

	err = g.GetStream(g.ntripClient.MountPoint, g.ntripClient.MaxConnectAttempts)
	if err != nil {
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
	return g.ntripStatus, nil
}

// GetPosition returns the current geographic location of the MOVEMENTSENSOR.
func (g *RTKMovementSensor) GetPosition(ctx context.Context) (*geo.Point, float64, error) {
	return g.nmeamovementsensor.GetPosition(ctx)
}

// GetLinearVelocity passthrough.
func (g *RTKMovementSensor) GetLinearVelocity(ctx context.Context) (r3.Vector, error) {
	return g.nmeamovementsensor.GetLinearVelocity(ctx)
}

// GetAngularVelocity passthrough.
func (g *RTKMovementSensor) GetAngularVelocity(ctx context.Context) (spatialmath.AngularVelocity, error) {
	return g.nmeamovementsensor.GetAngularVelocity(ctx)
}

// GetCompassHeading passthrough.
func (g *RTKMovementSensor) GetCompassHeading(ctx context.Context) (float64, error) {
	return g.nmeamovementsensor.GetCompassHeading(ctx)
}

// GetOrientation passthrough.
func (g *RTKMovementSensor) GetOrientation(ctx context.Context) (spatialmath.Orientation, error) {
	return g.nmeamovementsensor.GetOrientation(ctx)
}

// ReadFix passthrough.
func (g *RTKMovementSensor) ReadFix(ctx context.Context) (int, error) {
	return g.nmeamovementsensor.ReadFix(ctx)
}

// GetProperties passthrough.
func (g *RTKMovementSensor) GetProperties(ctx context.Context) (*movementsensor.Properties, error) {
	return g.nmeamovementsensor.GetProperties(ctx)
}

// GetAccuracy passthrough.
func (g *RTKMovementSensor) GetAccuracy(ctx context.Context) (map[string]float32, error) {
	return g.nmeamovementsensor.GetAccuracy(ctx)
}

// GetReadings will use the default MovementSensor GetReadings if not provided.
func (g *RTKMovementSensor) GetReadings(ctx context.Context) (map[string]interface{}, error) {
	readings, err := movementsensor.GetReadings(ctx, g)
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

	return nil
}
