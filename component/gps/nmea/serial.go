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
	"go.viam.com/utils/serial"

	"go.viam.com/rdk/component/generic"
	"go.viam.com/rdk/component/gps"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/robot"
)

func init() {
	registry.RegisterComponent(
		gps.Subtype,
		"nmea-serial",
		registry.Component{Constructor: func(
			ctx context.Context,
			r robot.Robot,
			config config.Component,
			logger golog.Logger,
		) (interface{}, error) {
			return newSerialNMEAGPS(ctx, config, logger)
		}})
}

type serialNMEAGPS struct {
	generic.Unimplemented
	mu     sync.RWMutex
	dev    io.ReadWriteCloser
	logger golog.Logger

	data                    gpsData
	cancelCtx               context.Context
	cancelFunc              func()
	activeBackgroundWorkers sync.WaitGroup
}

const (
	pathAttrName = "path"
)

func newSerialNMEAGPS(ctx context.Context, config config.Component, logger golog.Logger) (gps.LocalGPS, error) {
	serialPath := config.Attributes.String(pathAttrName)
	if serialPath == "" {
		return nil, fmt.Errorf("serialNMEAGPS expected non-empty string for %q", pathAttrName)
	}
	dev, err := serial.Open(serialPath)
	if err != nil {
		return nil, err
	}

	cancelCtx, cancelFunc := context.WithCancel(ctx)

	g := &serialNMEAGPS{dev: dev, cancelCtx: cancelCtx, cancelFunc: cancelFunc, logger: logger}

	g.Start(ctx)

	return g, nil
}

func (g *serialNMEAGPS) Start(ctx context.Context) {
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
			err = g.data.parseAndUpdate(line)
			g.mu.Unlock()
			if err != nil {
				g.logger.Debugf("can't parse nmea %s : %s", line, err)
			}
		}
	})
}

func (g *serialNMEAGPS) ReadLocation(ctx context.Context) (*geo.Point, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.data.location, nil
}

func (g *serialNMEAGPS) ReadAltitude(ctx context.Context) (float64, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.data.alt, nil
}

func (g *serialNMEAGPS) ReadSpeed(ctx context.Context) (float64, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.data.speed, nil
}

func (g *serialNMEAGPS) ReadSatellites(ctx context.Context) (int, int, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.data.satsInUse, g.data.satsInView, nil
}

func (g *serialNMEAGPS) ReadAccuracy(ctx context.Context) (float64, float64, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.data.hDOP, g.data.vDOP, nil
}

func (g *serialNMEAGPS) ReadValid(ctx context.Context) (bool, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.data.valid, nil
}

func (g *serialNMEAGPS) Close() error {
	g.cancelFunc()
	g.activeBackgroundWorkers.Wait()
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.dev != nil {
		if err := g.dev.Close(); err != nil {
			return err
		}
		g.dev = nil
	}
	return nil
}

// toPoint converts a nmea.GLL to a geo.Point.
func toPoint(a nmea.GLL) *geo.Point {
	return geo.NewPoint(a.Latitude, a.Longitude)
}
