// Package gpsrtkserial implements a gps using serial connection
package gpsrtkserial

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
	"github.com/edaniels/golog"
	"github.com/go-gnss/rtcm/rtcm3"
	"github.com/golang/geo/r3"
	slib "github.com/jacobsa/go-serial/serial"
	geo "github.com/kellydunn/golang-geo"
	"go.viam.com/utils"

	"go.viam.com/rdk/components/movementsensor"
	gpsnmea "go.viam.com/rdk/components/movementsensor/gpsnmea"
	rtk "go.viam.com/rdk/components/movementsensor/gpsrtk"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/spatialmath"
)

var rtkmodel = resource.DefaultModelFamily.WithModel("gps-rtk-serial")

var (
	errConnectionTypeValidation = fmt.Errorf("only serial is supported connection types for %s", rtkmodel.Name)
	errInputProtocolValidation  = fmt.Errorf("only serial is supported input protocols for %s", rtkmodel.Name)
)

const (
	serialStr = "serial"
	ntripStr  = "ntrip"
)

// Config is used for converting NMEA MovementSensor with RTK capabilities config attributes.
type Config struct {
	NmeaDataSource           string `json:"nmea_data_source"`
	SerialPath               string `json:"serial_path"`
	SerialBaudRate           int    `json:"serial_baud_rate,omitempty"`
	SerialCorrectionPath     string `json:"serial_correction_path,omitempty"`
	SerialCorrectionBaudRate int    `json:"serial_correction_baud_rate,omitempty"`

	NtripURL             string `json:"ntrip_url"`
	NtripConnectAttempts int    `json:"ntrip_connect_attempts,omitempty"`
	NtripMountpoint      string `json:"ntrip_mountpoint,omitempty"`
	NtripPass            string `json:"ntrip_password,omitempty"`
	NtripUser            string `json:"ntrip_username,omitempty"`
	NtripInputProtocol   string `json:"ntrip_input_protocol,omitempty"`
}

// Validate ensures all parts of the config are valid.
func (cfg *Config) Validate(path string) ([]string, error) {
	err := cfg.validateNmeaDataSource(path)
	if err != nil {
		return nil, err
	}

	err = cfg.validateNtripInputProtocol()
	if err != nil {
		return nil, err
	}

	err = cfg.ValidateNtrip(path)
	if err != nil {
		return nil, err
	}

	return nil, nil
}

func (cfg *Config) validateNmeaDataSource(path string) error {
	switch strings.ToLower(cfg.NmeaDataSource) {
	case serialStr:
		return cfg.validateSerialPath(path)
	case "":
		return utils.NewConfigValidationFieldRequiredError(path, "nmea_data_source")
	default:
		return errConnectionTypeValidation
	}
}

// validateNtripInputProtocol validates protocols accepted by this package.
func (cfg *Config) validateNtripInputProtocol() error {
	if cfg.NtripInputProtocol == serialStr {
		return nil
	}
	return errInputProtocolValidation
}

// validateSerialPath ensures all parts of the config are valid.
func (cfg *Config) validateSerialPath(path string) error {
	if cfg.SerialPath == "" {
		return utils.NewConfigValidationFieldRequiredError(path, "serial_path")
	}
	return nil
}

// ValidateNtrip ensures all parts of the config are valid.
func (cfg *Config) ValidateNtrip(path string) error {
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
		rtkmodel,
		resource.Registration[movementsensor.MovementSensor, *Config]{
			Constructor: newRTKSerial,
		})
}

// RTKSerial is an nmea movementsensor model that can intake RTK correction data.
type RTKSerial struct {
	resource.Named
	resource.AlwaysRebuild
	logger     golog.Logger
	cancelCtx  context.Context
	cancelFunc func()

	activeBackgroundWorkers sync.WaitGroup

	ntripMu     sync.Mutex
	ntripClient *rtk.NtripInfo
	ntripStatus bool

	err           movementsensor.LastError
	lastposition  movementsensor.LastPosition
	InputProtocol string

	Nmeamovementsensor gpsnmea.NmeaMovementSensor
	CorrectionWriter   io.ReadWriteCloser
	Writepath          string
	Wbaud              int
}

func newRTKSerial(
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
	g := &RTKSerial{
		Named:        conf.ResourceName().AsNamed(),
		cancelCtx:    cancelCtx,
		cancelFunc:   cancelFunc,
		logger:       logger,
		err:          movementsensor.NewLastError(1, 1),
		lastposition: movementsensor.NewLastPosition(),
	}

	ntripConfig := &rtk.NtripConfig{
		NtripURL:             newConf.NtripURL,
		NtripUser:            newConf.NtripUser,
		NtripPass:            newConf.NtripPass,
		NtripMountpoint:      newConf.NtripMountpoint,
		NtripConnectAttempts: newConf.NtripConnectAttempts,
	}

	g.InputProtocol = strings.ToLower(newConf.NtripInputProtocol)

	nmeaConf := &gpsnmea.Config{
		ConnectionType: newConf.NmeaDataSource,
	}

	// Init NMEAMovementSensor
	switch strings.ToLower(newConf.NmeaDataSource) {
	case serialStr:
		var err error
		nmeaConf.SerialConfig = &gpsnmea.SerialConfig{
			SerialPath:               newConf.SerialPath,
			SerialBaudRate:           newConf.SerialBaudRate,
			SerialCorrectionPath:     newConf.SerialCorrectionPath,
			SerialCorrectionBaudRate: newConf.SerialCorrectionBaudRate,
		}
		g.Nmeamovementsensor, err = gpsnmea.NewSerialGPSNMEA(ctx, conf.ResourceName(), nmeaConf, logger)
		if err != nil {
			return nil, err
		}
	default:
		// Invalid protocol
		return nil, fmt.Errorf("%s is not a valid connection type", newConf.NmeaDataSource)
	}

	g.ntripClient, err = rtk.NewNtripInfo(ntripConfig, g.logger)
	if err != nil {
		return nil, err
	}

	// baud rate
	if newConf.SerialBaudRate == 0 {
		newConf.SerialBaudRate = 38400
		g.logger.Info("serial_baud_rate using default baud rate 38400")
	}
	g.Wbaud = newConf.SerialBaudRate
	g.Writepath = newConf.SerialPath

	if err := g.start(); err != nil {
		return nil, err
	}
	return g, g.err.Get()
}

func (g *RTKSerial) start() error {
	if err := g.Nmeamovementsensor.Start(g.cancelCtx); err != nil {
		g.lastposition.GetLastPosition()
		return err
	}
	g.activeBackgroundWorkers.Add(1)
	utils.PanicCapturingGo(g.receiveAndWriteSerial)
	return g.err.Get()
}

// Connect attempts to connect to ntrip client until successful connection or timeout.
func (g *RTKSerial) Connect(casterAddr, user, pwd string, maxAttempts int) error {
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
func (g *RTKSerial) GetStream(mountPoint string, maxAttempts int) error {
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

// receiveAndWriteSerial connects to NTRIP receiver and sends correction stream to the MovementSensor through serial.
func (g *RTKSerial) receiveAndWriteSerial() {
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
func (g *RTKSerial) NtripStatus() (bool, error) {
	g.ntripMu.Lock()
	defer g.ntripMu.Unlock()
	return g.ntripStatus, g.err.Get()
}

// Position returns the current geographic location of the MOVEMENTSENSOR.
func (g *RTKSerial) Position(ctx context.Context, extra map[string]interface{}) (*geo.Point, float64, error) {
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
func (g *RTKSerial) LinearVelocity(ctx context.Context, extra map[string]interface{}) (r3.Vector, error) {
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
func (g *RTKSerial) LinearAcceleration(ctx context.Context, extra map[string]interface{}) (r3.Vector, error) {
	lastError := g.err.Get()
	if lastError != nil {
		return r3.Vector{}, lastError
	}
	return g.Nmeamovementsensor.LinearAcceleration(ctx, extra)
}

// AngularVelocity passthrough.
func (g *RTKSerial) AngularVelocity(ctx context.Context, extra map[string]interface{}) (spatialmath.AngularVelocity, error) {
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
func (g *RTKSerial) CompassHeading(ctx context.Context, extra map[string]interface{}) (float64, error) {
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
func (g *RTKSerial) Orientation(ctx context.Context, extra map[string]interface{}) (spatialmath.Orientation, error) {
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
func (g *RTKSerial) ReadFix(ctx context.Context) (int, error) {
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
func (g *RTKSerial) Properties(ctx context.Context, extra map[string]interface{}) (*movementsensor.Properties, error) {
	lastError := g.err.Get()
	if lastError != nil {
		return &movementsensor.Properties{}, lastError
	}

	return g.Nmeamovementsensor.Properties(ctx, extra)
}

// Accuracy passthrough.
func (g *RTKSerial) Accuracy(ctx context.Context, extra map[string]interface{}) (map[string]float32, error) {
	lastError := g.err.Get()
	if lastError != nil {
		return map[string]float32{}, lastError
	}

	return g.Nmeamovementsensor.Accuracy(ctx, extra)
}

// Readings will use the default MovementSensor Readings if not provided.
func (g *RTKSerial) Readings(ctx context.Context, extra map[string]interface{}) (map[string]interface{}, error) {
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

// Close shuts down the RTKSerial.
func (g *RTKSerial) Close(ctx context.Context) error {
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
