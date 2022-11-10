// Package gpsnmea implements an NMEA serial gps.
package gpsnmea

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"sync"

	"github.com/adrianmo/go-nmea"
	"github.com/edaniels/golog"
	"github.com/golang/geo/r3"
	"github.com/jacobsa/go-serial/serial"
	geo "github.com/kellydunn/golang-geo"
	"github.com/pkg/errors"
	"go.viam.com/utils"

	"go.viam.com/rdk/components/generic"
	"go.viam.com/rdk/components/movementsensor"
	"go.viam.com/rdk/spatialmath"
)

var errNilLocation = errors.New("nil gps location, check nmea message parsing")

// SerialNMEAMovementSensor allows the use of any MovementSensor chip that communicates over serial.
type SerialNMEAMovementSensor struct {
	generic.Unimplemented
	mu                      sync.RWMutex
	cancelCtx               context.Context
	cancelFunc              func()
	logger                  golog.Logger
	data                    gpsData
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

// NewSerialGPSNMEA gps that communicates over serial.
func NewSerialGPSNMEA(ctx context.Context, attr *AttrConfig, logger golog.Logger) (NmeaMovementSensor, error) {
	serialPath := attr.SerialAttrConfig.SerialPath
	if serialPath == "" {
		return nil, fmt.Errorf("SerialNMEAMovementSensor expected non-empty string for %q", attr.SerialAttrConfig.SerialPath)
	}
	correctionPath := attr.SerialAttrConfig.SerialCorrectionPath
	if correctionPath == "" {
		correctionPath = serialPath
		logger.Infof("SerialNMEAMovementSensor: correction_path using path: %s", correctionPath)
	}
	baudRate := attr.SerialAttrConfig.SerialBaudRate
	if baudRate == 0 {
		baudRate = 9600
		logger.Info("SerialNMEAMovementSensor: serial_baud_rate using default 9600")
	}
	correctionBaudRate := attr.SerialAttrConfig.SerialCorrectionBaudRate
	if correctionBaudRate == 0 {
		correctionBaudRate = baudRate
		logger.Infof("SerialNMEAMovementSensor: correction_baud using baud_rate: %d", baudRate)
	}
	disableNmea := attr.DisableNMEA
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
	}

	if err := g.Start(ctx); err != nil {
		g.logger.Errorf("Did not create nmea gps with err %#v", err.Error())
	}

	return g, err
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
					g.logger.Warnf("can't parse nmea sentence: %#v", err)
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

// Position position, altitide.
func (g *SerialNMEAMovementSensor) Position(ctx context.Context) (*geo.Point, float64, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()
	if g.data.location == nil {
		return geo.NewPoint(0, 0), 0, errNilLocation
	}
	return g.data.location, g.data.alt, g.lastError
}

// Accuracy returns the accuracy, hDOP and vDOP.
func (g *SerialNMEAMovementSensor) Accuracy(ctx context.Context) (map[string]float32, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return map[string]float32{"hDOP": float32(g.data.hDOP), "vDOP": float32(g.data.vDOP)}, nil
}

// LinearVelocity linear velocity.
func (g *SerialNMEAMovementSensor) LinearVelocity(ctx context.Context) (r3.Vector, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return r3.Vector{X: 0, Y: g.data.speed, Z: 0}, nil
}

// AngularVelocity angularvelocity.
func (g *SerialNMEAMovementSensor) AngularVelocity(ctx context.Context) (spatialmath.AngularVelocity, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return spatialmath.AngularVelocity{}, movementsensor.ErrMethodUnimplementedAngularVelocity
}

// Orientation orientation.
func (g *SerialNMEAMovementSensor) Orientation(ctx context.Context) (spatialmath.Orientation, error) {
	return nil, movementsensor.ErrMethodUnimplementedOrientation
}

// CompassHeading 0->360.
func (g *SerialNMEAMovementSensor) CompassHeading(ctx context.Context) (float64, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return 0, movementsensor.ErrMethodUnimplementedCompassHeading
}

// ReadFix returns Fix quality of MovementSensor measurements.
func (g *SerialNMEAMovementSensor) ReadFix(ctx context.Context) (int, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.data.fixQuality, nil
}

// Readings will use return all of the MovementSensor Readings.
func (g *SerialNMEAMovementSensor) Readings(ctx context.Context, extra map[string]interface{}) (map[string]interface{}, error) {
	readings, err := movementsensor.Readings(ctx, g)
	if err != nil {
		return nil, g.lastError
	}

	fix, err := g.ReadFix(ctx)
	if err != nil {
		return nil, err
	}

	readings["fix"] = fix

	return readings, g.lastError
}

// Properties what do I do!
func (g *SerialNMEAMovementSensor) Properties(ctx context.Context) (*movementsensor.Properties, error) {
	return &movementsensor.Properties{
		LinearVelocitySupported: true,
		PositionSupported:       true,
	}, g.lastError
}

// Close shuts down the SerialNMEAMovementSensor.
func (g *SerialNMEAMovementSensor) Close() error {
	g.logger.Debug("Closing SerialNMEAMovementSensor")
	g.cancelFunc()
	defer g.activeBackgroundWorkers.Wait()

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
