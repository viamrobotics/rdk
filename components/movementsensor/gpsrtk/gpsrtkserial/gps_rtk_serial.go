// Package gpsrtkserial implements a gps using serial connection
package gpsrtk

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"strings"
	"sync"

	"github.com/edaniels/golog"
	"github.com/golang/geo/r3"
	geo "github.com/kellydunn/golang-geo"
	"go.viam.com/rdk/components/movementsensor"
	gpsnmea "go.viam.com/rdk/components/movementsensor/gpsnmea"
	rtk "go.viam.com/rdk/components/movementsensor/gpsrtk"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/spatialmath"
	"go.viam.com/utils"
)

var rtkmodel = resource.DefaultModelFamily.WithModel("gps-rtk-serial")

var (
	errCorrectionSourceValidation = fmt.Errorf("only serial is supported correction sources for %s", rtkmodel.Name)
	errConnectionTypeValidation   = fmt.Errorf("only serial is supported connection types for %s", rtkmodel.Name)
	errInputProtocolValidation    = fmt.Errorf("only serial is supported input protocols for %s", rtkmodel.Name)
)

const (
	serialStr = "serial"
	ntripStr  = "ntrip"
)

type Config struct {
	NmeaDataSource           string `json:"nmea_data_source"`
	SerialPath               string `json:"serial_path"`
	SerialBaudRate           int    `json:"serial_baud_rate,omitempty"`
	SerialCorrectionPath     string `json:"serial_correction_path,omitempty"`
	SerialCorrectionBaudRate int    `json:"serial_correction_baud_rate,omitempty"`

	*NtripConfig `json:"ntrip_attributes,omitempty"`
}

// NtripConfig is used for converting attributes for a correction source.
type NtripConfig struct {
	NtripURL             string `json:"ntrip_url"`
	NtripConnectAttempts int    `json:"ntrip_connect_attempts,omitempty"`
	NtripMountpoint      string `json:"ntrip_mountpoint,omitempty"`
	NtripPass            string `json:"ntrip_password,omitempty"`
	NtripUser            string `json:"ntrip_username,omitempty"`
	NtripInputProtocol   string `json:"ntrip_input_protocol,omitempty"`
}

func (cfg *Config) Validate(path string) ([]string, error) {
	var deps []string

	dep, err := cfg.validateNmeaDataSource(path)
	if err != nil {
		return nil, err
	}
	if dep != nil {
		deps = append(deps, dep...)
	}

	if cfg.NmeaDataSource == ntripStr {
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

func (cfg *Config) validateNmeaDataSource(path string) ([]string, error) {
	switch strings.ToLower(cfg.NmeaDataSource) {
	case serialStr:
		return nil, cfg.ValidateSerialPath(path)
	case "":
		return nil, utils.NewConfigValidationFieldRequiredError(path, "connection_type")
	default:
		return nil, errConnectionTypeValidation
	}
}

// validateNtripInputProtocol validates protocols accepted by this package
func (cfg *Config) validateNtripInputProtocol(path string) ([]string, error) {

	switch cfg.NtripInputProtocol {
	case serialStr:
		return nil, cfg.ValidateSerialPath(path)
	default:
		return nil, errInputProtocolValidation
	}
}

// ValidateSerial ensures all parts of the config are valid.
func (cfg *Config) ValidateSerialPath(path string) error {
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
		rtkmodel,
		resource.Registration[movementsensor.MovementSensor, *Config]{})
}

// RTKSerial is an nmea movementsensor model that can intake RTK correction data
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

	err          movementsensor.LastError
	lastposition movementsensor.LastPosition

	Nmeamovementsensor gpsnmea.NmeaMovementSensor
	CorrectionWriter   io.ReadWriteCloser
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

	nmeaConf := &gpsnmea.Config{
		ConnectionType: newConf.NmeaDataSource,
	}

	// Init NMEAMovementSensor
	switch strings.ToLower(newConf.NmeaDataSource) {
	case serialStr:
		var err error
		nmeaConf.SerialConfig = &gpsnmea.SerialConfig{SerialPath: newConf.SerialPath,
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

	// Init ntripInfo from attibutes
	g.ntripClient, err = rtk.NewNtripInfo((*rtk.NtripConfig)(newConf.NtripConfig), g.logger)
	if err != nil {
		return nil, err
	}

	if err := g.start(); err != nil {
		return nil, err
	}
	return g, g.err.Get()
}

func (g *RTKSerial) start() error {

	return g.err.Get()
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

// Close shuts down the RTKSerial
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
