// Package nmea implements an NMEA serial gps.
package nmea

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

	"go.viam.com/rdk/component/generic"
	"go.viam.com/rdk/component/movementsensor"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/spatialmath"
)

// SerialAttrConfig is used for converting Serial NMEA MovementSensor config attributes.
type SerialAttrConfig struct {
	// Serial
	SerialPath     string `json:"path"`
	CorrectionPath string `json:"correction_path"`
}

// ValidateSerial ensures all parts of the config are valid.
func (config *SerialAttrConfig) ValidateSerial(path string) error {
	if len(config.SerialPath) == 0 {
		return errors.New("expected nonempty path")
	}

	return nil
}

func init() {
	registry.RegisterComponent(
		movementsensor.Subtype,
		"nmea-serial",
		registry.Component{Constructor: func(
			ctx context.Context,
			_ registry.Dependencies,
			config config.Component,
			logger golog.Logger,
		) (interface{}, error) {
			return newSerialNMEAMovementSensor(ctx, config, logger)
		}})
}

// SerialNMEAMovementSensor allows the use of any MovementSensor chip that communicates over serial.
type SerialNMEAMovementSensor struct {
	generic.Unimplemented
	mu                 sync.RWMutex
	dev                io.ReadWriteCloser
	logger             golog.Logger
	path               string
	correctionPath     string
	baudRate           uint
	correctionBaudRate uint
	disableNmea        bool

	data                    gpsData
	cancelCtx               context.Context
	cancelFunc              func()
	activeBackgroundWorkers sync.WaitGroup
}

const (
	pathAttrName           = "path"
	correctionAttrName     = "correction_path"
	baudRateName           = "baud_rate"
	correctionBaudRateName = "correction_baud"
	disableNmeaName        = "disable_nmea"
)

func newSerialNMEAMovementSensor(ctx context.Context, config config.Component, logger golog.Logger) (nmeaMovementSensor, error) {
	serialPath := config.Attributes.String(pathAttrName)
	if serialPath == "" {
		return nil, fmt.Errorf("SerialNMEAMovementSensor expected non-empty string for %q", pathAttrName)
	}
	correctionPath := config.Attributes.String(correctionAttrName)
	if correctionPath == "" {
		correctionPath = serialPath
		logger.Info("SerialNMEAMovementSensor: correction_path using path")
	}
	baudRate := config.Attributes.Int(baudRateName, 0)
	if baudRate == 0 {
		baudRate = 9600
		logger.Info("SerialNMEAMovementSensor: baud_rate using default 9600")
	}
	correctionBaudRate := config.Attributes.Int(correctionBaudRateName, 0)
	if correctionBaudRate == 0 {
		correctionBaudRate = baudRate
		logger.Info("SerialNMEAMovementSensor: correction_baud using baud_rate")
	}
	disableNmea := config.Attributes.Bool(disableNmeaName, false)
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
		data:               gpsData{},
	}

	g.Start(ctx)

	return g, nil
}

// Start begins reading nmea messages from module and updates gps data.
func (g *SerialNMEAMovementSensor) Start(ctx context.Context) {
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
					g.logger.Fatalf("can't read gps serial %s", err)
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
}

// GetCorrectionInfo returns the serial path that takes in rtcm corrections and baudrate for reading.
func (g *SerialNMEAMovementSensor) GetCorrectionInfo() (string, uint) {
	return g.correctionPath, g.correctionBaudRate
}

// GetPosition position, altitide, accuracy.
func (g *SerialNMEAMovementSensor) GetPosition(ctx context.Context) (*geo.Point, float64, *geo.Point, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.data.location, g.data.alt, geo.NewPoint(g.data.hDOP, g.data.vDOP), nil
}

// GetLinearVelocity linear velocity.
func (g *SerialNMEAMovementSensor) GetLinearVelocity(ctx context.Context) (r3.Vector, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return r3.Vector{0, g.data.speed, 0}, nil
}

// GetAngularVelocity angularvelocity.
func (g *SerialNMEAMovementSensor) GetAngularVelocity(ctx context.Context) (spatialmath.AngularVelocity, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return spatialmath.AngularVelocity{}, nil
}

// GetOrientation orientation.
func (g *SerialNMEAMovementSensor) GetOrientation(ctx context.Context) (spatialmath.Orientation, error) {
	return nil, nil
}

// GetCompassHeading 0->360.
func (g *SerialNMEAMovementSensor) GetCompassHeading(ctx context.Context) (float64, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return 0, nil
}

// ReadFix returns Fix quality of MovementSensor measurements.
func (g *SerialNMEAMovementSensor) ReadFix(ctx context.Context) (int, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.data.fixQuality, nil
}

// GetReadings will use return all of the MovementSensor Readings.
func (g *SerialNMEAMovementSensor) GetReadings(ctx context.Context) ([]interface{}, error) {
	readings, err := movementsensor.GetReadings(ctx, g)
	if err != nil {
		return nil, err
	}

	fix, err := g.ReadFix(ctx)
	if err != nil {
		return nil, err
	}

	readings = append(readings, fix)

	return readings, nil
}

// GetProperties what do I do!
func (g *SerialNMEAMovementSensor) GetProperties(ctx context.Context) (*movementsensor.Properties, error) {
	return &movementsensor.Properties{
		LinearVelocitySupported: true,
		PositionSupported:       true,
	}, nil
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
	return nil
}

// toPoint converts a nmea.GLL to a geo.Point.
func toPoint(a nmea.GLL) *geo.Point {
	return geo.NewPoint(a.Latitude, a.Longitude)
}
