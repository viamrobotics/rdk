// Package nmea implements an NMEA serial gps.
package nmea

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"sync"

	"github.com/adrianmo/go-nmea"
	"github.com/edaniels/golog"
	geo "github.com/kellydunn/golang-geo"
	"go.viam.com/utils"

	"go.viam.com/rdk/config"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/robot"
	"go.viam.com/rdk/sensor"
	"go.viam.com/rdk/sensor/gps"
	"go.viam.com/rdk/serial"
)

func init() {
	registry.RegisterSensor(
		gps.Type,
		"nmea-serial",
		registry.Sensor{Constructor: func(
			ctx context.Context,
			r robot.Robot,
			config config.Component,
			logger golog.Logger,
		) (sensor.Sensor, error) {
			return newSerialNMEAGPS(config, logger)
		}})
}

type serialNMEAGPS struct {
	mu     sync.RWMutex
	dev    io.ReadWriteCloser
	logger golog.Logger

	data gpsData

	cancelCtx               context.Context
	cancelFunc              func()
	activeBackgroundWorkers sync.WaitGroup
}

const pathAttrName = "path"

func newSerialNMEAGPS(config config.Component, logger golog.Logger) (gps.GPS, error) {
	serialPath := config.Attributes.String(pathAttrName)
	if serialPath == "" {
		return nil, fmt.Errorf("serialNMEAGPS expected non-empty string for %q", pathAttrName)
	}
	dev, err := serial.Open(serialPath)
	if err != nil {
		return nil, err
	}

	cancelCtx, cancelFunc := context.WithCancel(context.Background())

	g := &serialNMEAGPS{dev: dev, cancelCtx: cancelCtx, cancelFunc: cancelFunc, logger: logger}
	g.Start()

	return g, nil
}

func (g *serialNMEAGPS) Start() {
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

			line, err := r.ReadString('\n')
			if err != nil {
				g.logger.Fatalf("can't read gps serial %s", err)
			}

			// Update our struct's gps data in-place
			g.mu.Lock()
			err = parseAndUpdate(line, &g.data)
			g.mu.Unlock()
			if err != nil {
				g.logger.Debugf("can't parse nmea %s : %s", line, err)
			}
		}
	})
}

func (g *serialNMEAGPS) Readings(ctx context.Context) ([]interface{}, error) {
	loc, err := g.Location(ctx)
	if err != nil {
		return nil, err
	}
	return []interface{}{loc}, nil
}

func (g *serialNMEAGPS) Location(ctx context.Context) (*geo.Point, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.data.location, nil
}

func (g *serialNMEAGPS) Altitude(ctx context.Context) (float64, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.data.alt, nil
}

func (g *serialNMEAGPS) Speed(ctx context.Context) (float64, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.data.speed, nil
}

func (g *serialNMEAGPS) Satellites(ctx context.Context) (int, int, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.data.satsInUse, g.data.satsInView, nil
}

func (g *serialNMEAGPS) Accuracy(ctx context.Context) (float64, float64, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.data.hDOP, g.data.vDOP, nil
}

func (g *serialNMEAGPS) Valid(ctx context.Context) (bool, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.data.valid, nil
}

func (g *serialNMEAGPS) Close() error {
	g.cancelFunc()
	g.activeBackgroundWorkers.Wait()
	return g.dev.Close()
}

// Desc returns that this is a GPS.
func (g *serialNMEAGPS) Desc() sensor.Description {
	return sensor.Description{gps.Type, ""}
}

// toPoint converts a nmea.GLL to a geo.Point.
func toPoint(a nmea.GLL) *geo.Point {
	return geo.NewPoint(a.Latitude, a.Longitude)
}
