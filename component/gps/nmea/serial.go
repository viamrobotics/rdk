// Package nmea implements an NMEA serial gps.
package nmea

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"sync"

	"github.com/adrianmo/go-nmea"
	"github.com/edaniels/golog"
	"github.com/go-gnss/ntrip"
	slib "github.com/jacobsa/go-serial/serial"
	geo "github.com/kellydunn/golang-geo"
	"go.uber.org/multierr"
	"go.viam.com/utils"
	"go.viam.com/utils/serial"

	"go.viam.com/rdk/component/gps"
	"go.viam.com/rdk/component/sensor"
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
	mu             sync.RWMutex
	dev            io.ReadWriteCloser
	logger         golog.Logger
	ntripURL       string
	ntripUsername  string
	ntripPassword  string
	ntripWritepath string
	ntripWbaud     int
	data           gpsData

	cancelCtx               context.Context
	cancelFunc              func()
	activeBackgroundWorkers sync.WaitGroup
}

const (
	pathAttrName      = "path"
	ntripAddrAttrName = "ntripAddr"
	ntripUserAttrName = "ntripUsername"
	ntripPassAttrName = "ntripPassword"
	ntripPathAttrName = "ntripPath"
	ntripBaudAttrName = "ntripBaud"
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
	g.ntripURL = config.Attributes.String(ntripAddrAttrName)
	if g.ntripURL != "" {
		g.ntripUsername = config.Attributes.String(ntripUserAttrName)
		g.ntripPassword = config.Attributes.String(ntripPassAttrName)
		g.ntripWritepath = config.Attributes.String(ntripPathAttrName)
		g.ntripWbaud = config.Attributes.Int(ntripBaudAttrName, g.ntripWbaud)
		if g.ntripWritepath == "" {
			g.ntripWritepath = serialPath
		}
		g.Start()
		g.startNtripClientRequest()
	} else {
		g.Start()
	}

	return g, nil
}

func (g *serialNMEAGPS) fetchNtripAndUpdate() error {
	// setup the ntrip client and write the RTCM data stream to the gps
	// talk to the gps network, looking for mount points
	req, err := ntrip.NewClientRequest(g.ntripURL)
	if err != nil {
		return err
	}
	req.SetBasicAuth(g.ntripUsername, g.ntripPassword)
	req = req.WithContext(g.cancelCtx)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		err = utils.FilterOutError(err, context.Canceled)
		if err == nil {
			g.logger.Debug("error send ntrip request: context cancelled")
		}
		return err
	} else if resp.StatusCode != http.StatusOK {
		return errors.New("received non-200 response code: " + strconv.Itoa(resp.StatusCode))
	}

	// setup port to write to
	options := slib.OpenOptions{
		BaudRate:        uint(g.ntripWbaud),
		DataBits:        8,
		StopBits:        1,
		MinimumReadSize: 1,
	}
	options.PortName = g.ntripWritepath
	port, err := slib.Open(options)
	if err != nil {
		return multierr.Combine(err, resp.Body.Close())
	}
	w := bufio.NewWriter(port)

	// Read from resp.Body until EOF
	r := io.TeeReader(resp.Body, w)
	_, err = io.ReadAll(r)

	if err != nil {
		return multierr.Combine(err, resp.Body.Close())
	}

	return nil
}

func (g *serialNMEAGPS) startNtripClientRequest() {
	g.activeBackgroundWorkers.Add(1)
	utils.PanicCapturingGo(func() {
		defer g.activeBackgroundWorkers.Done()

		// loop to reconnect in case something breaks
		for {
			select {
			case <-g.cancelCtx.Done():
				return
			default:
			}
			err := g.fetchNtripAndUpdate()
			if err != nil {
				g.logger.Errorf("Error with ntrip client %s", err)
			}
		}
	})
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

// Desc returns that this is a GPS.
func (g *serialNMEAGPS) Desc() sensor.Description {
	return sensor.Description{sensor.Type(gps.SubtypeName), ""}
}

// toPoint converts a nmea.GLL to a geo.Point.
func toPoint(a nmea.GLL) *geo.Point {
	return geo.NewPoint(a.Latitude, a.Longitude)
}
