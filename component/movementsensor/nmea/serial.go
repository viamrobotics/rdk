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
	"github.com/jacobsa/go-serial/serial"
	geo "github.com/kellydunn/golang-geo"
	"go.viam.com/utils"

	"go.viam.com/rdk/component/generic"
	"go.viam.com/rdk/component/gps"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/registry"
)

// SerialAttrConfig is used for converting Serial NMEA GPS config attributes.
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
		gps.Subtype,
		"nmea-serial",
		registry.Component{Constructor: func(
			ctx context.Context,
			_ registry.Dependencies,
			config config.Component,
			logger golog.Logger,
		) (interface{}, error) {
			return newSerialNMEAGPS(ctx, config, logger)
		}})
}

// SerialNMEAGPS allows the use of any GPS chip that communicates over serial.
type SerialNMEAGPS struct {
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

func newSerialNMEAGPS(ctx context.Context, config config.Component, logger golog.Logger) (nmeaGPS, error) {
	serialPath := config.Attributes.String(pathAttrName)
	if serialPath == "" {
		return nil, fmt.Errorf("SerialNMEAGPS expected non-empty string for %q", pathAttrName)
	}
	correctionPath := config.Attributes.String(correctionAttrName)
	if correctionPath == "" {
		correctionPath = serialPath
		logger.Info("SerialNMEAGPS: correction_path using path")
	}
	baudRate := config.Attributes.Int(baudRateName, 0)
	if baudRate == 0 {
		baudRate = 9600
		logger.Info("SerialNMEAGPS: baud_rate using default 9600")
	}
	correctionBaudRate := config.Attributes.Int(correctionBaudRateName, 0)
	if correctionBaudRate == 0 {
		correctionBaudRate = baudRate
		logger.Info("SerialNMEAGPS: correction_baud using baud_rate")
	}
	disableNmea := config.Attributes.Bool(disableNmeaName, false)
	if disableNmea {
		logger.Info("SerialNMEAGPS: NMEA reading disabled")
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

	g := &SerialNMEAGPS{
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
func (g *SerialNMEAGPS) Start(ctx context.Context) {
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
func (g *SerialNMEAGPS) GetCorrectionInfo() (string, uint) {
	return g.correctionPath, g.correctionBaudRate
}

// ReadLocation returns the current geographic location of the GPS.
func (g *SerialNMEAGPS) ReadLocation(ctx context.Context) (*geo.Point, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.data.location, nil
}

// ReadAltitude returns the current altitude of the GPS.
func (g *SerialNMEAGPS) ReadAltitude(ctx context.Context) (float64, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.data.alt, nil
}

// ReadSpeed returns the current speed of the GPS.
func (g *SerialNMEAGPS) ReadSpeed(ctx context.Context) (float64, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.data.speed, nil
}

// ReadSatellites returns the number of satellites that are currently visible to the GPS.
func (g *SerialNMEAGPS) ReadSatellites(ctx context.Context) (int, int, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.data.satsInUse, g.data.satsInView, nil
}

// ReadAccuracy returns how accurate the lat/long readings are.
func (g *SerialNMEAGPS) ReadAccuracy(ctx context.Context) (float64, float64, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.data.hDOP, g.data.vDOP, nil
}

// ReadValid returns whether or not the GPS is currently reading valid measurements.
func (g *SerialNMEAGPS) ReadValid(ctx context.Context) (bool, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.data.valid, nil
}

// ReadFix returns Fix quality of GPS measurements.
func (g *SerialNMEAGPS) ReadFix(ctx context.Context) (int, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.data.fixQuality, nil
}

// GetReadings will use return all of the GPS Readings.
func (g *SerialNMEAGPS) GetReadings(ctx context.Context) ([]interface{}, error) {
	readings, err := gps.GetReadings(ctx, g)
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

// Close shuts down the SerialNMEAGPS.
func (g *SerialNMEAGPS) Close() error {
	g.logger.Debug("Closing SerialNMEAGPS")
	g.cancelFunc()
	g.activeBackgroundWorkers.Wait()
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.dev != nil {
		if err := g.dev.Close(); err != nil {
			return err
		}
		g.dev = nil
		g.logger.Debug("SerialNMEAGPS Closed")
	}
	return nil
}

// toPoint converts a nmea.GLL to a geo.Point.
func toPoint(a nmea.GLL) *geo.Point {
	return geo.NewPoint(a.Latitude, a.Longitude)
}
