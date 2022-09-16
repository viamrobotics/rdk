// Package nmea implements an NMEA serial gps.
package gpsnmea

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"sync"

	"github.com/adrianmo/go-nmea"
	"github.com/edaniels/golog"
	"github.com/golang/geo/r3"
	"github.com/jacobsa/go-serial/serial"
	geo "github.com/kellydunn/golang-geo"
	"go.viam.com/utils"

	"go.viam.com/rdk/components/generic"
	"go.viam.com/rdk/components/movementsensor"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/spatialmath"
)

// SerialAttrConfig is used for converting Serial NMEA MovementSensor config attributes.
type SerialAttrConfig struct {
	// Serial
	SerialPath         string `json:"path"`
	BaudRate           int    `json:"baud_rate,omitempty"`
	CorrectionPath     string `json:"correction_path,omitempty"`
	CorrectionBaudRate int    `json:"correction_baud_rate,omitempty"`

	// *RTKAttrConfig `json:"rtk_attributes"`
}

// ValidateSerial ensures all parts of the config are valid.
func (config *SerialAttrConfig) ValidateSerial(path string) error {
	if len(config.SerialPath) == 0 {
		return errors.New("expected nonempty path")
	}

	return nil
}

// SerialNMEAMovementSensor allows the use of any MovementSensor chip that communicates over serial.
type SerialNMEAMovementSensor struct {
	generic.Unimplemented
	mu                      sync.RWMutex
	cancelCtx               context.Context
	cancelFunc              func()
	logger                  golog.Logger
	data                    GpsData
	activeBackgroundWorkers sync.WaitGroup

	disableNmea bool
	errMu       sync.Mutex
	lastError   error

	dev                io.ReadWriteCloser
	path               string
	baudRate           uint
	correctionBaudRate uint
	correctionPath     string
}

func NewSerialNMEAMovementSensor(ctx context.Context, config config.Component, logger golog.Logger) (NmeaMovementSensor, error) {
	conf, ok := config.ConvertedAttributes.(*AttrConfig)
	if !ok {
		return nil, errors.New("could not convert attributes from config")
	}

	serialPath := conf.SerialAttrConfig.SerialPath
	if serialPath == "" {
		return nil, fmt.Errorf("SerialNMEAMovementSensor expected non-empty string for %q", conf.SerialAttrConfig.SerialPath)
	}
	correctionPath := conf.SerialAttrConfig.CorrectionPath
	if correctionPath == "" {
		correctionPath = serialPath
		logger.Info("SerialNMEAMovementSensor: correction_path using path")
	}
	baudRate := conf.SerialAttrConfig.BaudRate
	if baudRate == 0 {
		baudRate = 9600
		logger.Info("SerialNMEAMovementSensor: baud_rate using default 9600")
	}
	correctionBaudRate := conf.SerialAttrConfig.CorrectionBaudRate
	if correctionBaudRate == 0 {
		correctionBaudRate = baudRate
		logger.Info("SerialNMEAMovementSensor: correction_baud using baud_rate")
	}
	disableNmea := conf.DisableNMEA
	if disableNmea {
		logger.Info("SerialNMEAMovementSensor: NMEA reading disabled")
	}

	options := serial.OpenOptions{
		PortName:        serialPath,
		BaudRate:        uint(baudRate),
		DataBits:        8,
		StopBits:        1,
		MinimumReadSize: 4,
	}

	dev, err := serial.Open(options)
	if err != nil {
		return nil, err
	}

	cancelCtx, cancelFunc := context.WithCancel(ctx)

	g := &SerialNMEAMovementSensor{
		dev:                dev,
		cancelCtx:          cancelCtx,
		cancelFunc:         cancelFunc,
		logger:             logger,
		path:               serialPath,
		correctionPath:     correctionPath,
		baudRate:           uint(baudRate),
		correctionBaudRate: uint(correctionBaudRate),
		disableNmea:        disableNmea,
		data:               GpsData{},
	}

	if err := g.Start(ctx); err != nil {
		return nil, err
	}

	return g, g.lastError
}

func (g *SerialNMEAMovementSensor) setLastError(err error) {
	g.errMu.Lock()
	defer g.errMu.Unlock()

	g.lastError = err
}

// Start begins reading nmea messages from module and updates gps data.
func (g *SerialNMEAMovementSensor) Start(ctx context.Context) error {
	g.activeBackgroundWorkers.Add(1)
	utils.PanicCapturingGo(func() {
		defer g.activeBackgroundWorkers.Done()
		r := bufio.NewReader(g.dev)
		for {
			select {
			case <-g.cancelCtx.Done():
				return
			default:
			}

			if !g.disableNmea {
				line, err := r.ReadString('\n')
				if err != nil {
					g.logger.Errorf("can't read gps serial %s", err)
					g.setLastError(err)
					return
				}
				// Update our struct's gps data in-place
				g.mu.Lock()
				err = g.data.parseAndUpdate(line)
				g.mu.Unlock()
				if err != nil {
					g.logger.Debugf("can't parse nmea %s : %s", line, err)
				}
			}
		}
	})

	return g.lastError
}

// GetCorrectionInfo returns the serial path that takes in rtcm corrections and baudrate for reading.
func (g *SerialNMEAMovementSensor) GetCorrectionInfo() (string, uint) {
	return g.correctionPath, g.correctionBaudRate
}

// GetPosition position, altitide.
func (g *SerialNMEAMovementSensor) GetPosition(ctx context.Context) (*geo.Point, float64, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.data.location, g.data.alt, g.lastError
}

// GetAccuracy returns the accuracy, hDOP and vDOP.
func (g *SerialNMEAMovementSensor) GetAccuracy(ctx context.Context) (map[string]float32, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return map[string]float32{"hDOP": float32(g.data.hDOP), "vDOP": float32(g.data.vDOP)}, g.lastError
}

// GetLinearVelocity linear velocity.
func (g *SerialNMEAMovementSensor) GetLinearVelocity(ctx context.Context) (r3.Vector, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return r3.Vector{X: 0, Y: g.data.speed, Z: 0}, g.lastError
}

// GetAngularVelocity angularvelocity.
func (g *SerialNMEAMovementSensor) GetAngularVelocity(ctx context.Context) (spatialmath.AngularVelocity, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return spatialmath.AngularVelocity{}, g.lastError
}

// GetOrientation orientation.
func (g *SerialNMEAMovementSensor) GetOrientation(ctx context.Context) (spatialmath.Orientation, error) {
	return nil, g.lastError
}

// GetCompassHeading 0->360.
func (g *SerialNMEAMovementSensor) GetCompassHeading(ctx context.Context) (float64, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return 0, g.lastError
}

// ReadFix returns Fix quality of MovementSensor measurements.
func (g *SerialNMEAMovementSensor) ReadFix(ctx context.Context) (int, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.data.fixQuality, g.lastError
}

// GetReadings will use return all of the MovementSensor Readings.
func (g *SerialNMEAMovementSensor) GetReadings(ctx context.Context) (map[string]interface{}, error) {
	readings, err := movementsensor.GetReadings(ctx, g)
	if err != nil {
		return nil, err
	}

	fix, err := g.ReadFix(ctx)
	if err != nil {
		return nil, err
	}

	readings["fix"] = fix

	return readings, g.lastError
}

// GetProperties what do I do!
func (g *SerialNMEAMovementSensor) GetProperties(ctx context.Context) (*movementsensor.Properties, error) {
	return &movementsensor.Properties{
		LinearVelocitySupported: true,
		PositionSupported:       true,
	}, g.lastError
}

// Close shuts down the SerialNMEAMovementSensor.
func (g *SerialNMEAMovementSensor) Close() error {
	g.logger.Debug("Closing SerialNMEAMovementSensor")
	g.cancelFunc()
	g.activeBackgroundWorkers.Wait()

	g.mu.Lock()
	defer g.mu.Unlock()
	if g.dev != nil {
		if err := g.dev.Close(); err != nil {
			return err
		}
		g.dev = nil
		g.logger.Debug("SerialNMEAMovementSensor Closed")
	}
	return g.lastError
}

// toPoint converts a nmea.GLL to a geo.Point.
func toPoint(a nmea.GLL) *geo.Point {
	return geo.NewPoint(a.Latitude, a.Longitude)
}
