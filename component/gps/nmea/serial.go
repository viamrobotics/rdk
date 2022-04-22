// Package nmea implements an NMEA serial gps.
package nmea

import (
	"bufio"
	"context"
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
	"github.com/pkg/errors"
	"go.uber.org/multierr"
	"go.viam.com/utils"
	"go.viam.com/utils/serial"

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
	mu     sync.RWMutex
	dev    io.ReadWriteCloser
	logger golog.Logger

	data                    gpsData
	ntripClient             ntripInfo
	cancelCtx               context.Context
	cancelFunc              func()
	activeBackgroundWorkers sync.WaitGroup
}

type ntripInfo struct {
	url       string
	username  string
	password  string
	writepath string
	wbaud     int
	sendNMEA  bool
	nmeaR     *io.PipeReader
	nmeaW     *io.PipeWriter
}

const (
	pathAttrName      = "path"
	ntripAddrAttrName = "ntrip_addr"
	ntripUserAttrName = "ntrip_username"
	ntripPassAttrName = "ntrip_password"
	ntripPathAttrName = "ntrip_path"
	ntripBaudAttrName = "ntrip_baud"
	ntripSendNmeaName = "ntrip_send_nmea"
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

	g.ntripClient.url = config.Attributes.String(ntripAddrAttrName)
	if g.ntripClient.url != "" {
		g.ntripClient.username = config.Attributes.String(ntripUserAttrName)
		if g.ntripClient.username == "" {
			g.logger.Info("ntrip_username set to empty")
		}
		g.ntripClient.password = config.Attributes.String(ntripPassAttrName)
		if g.ntripClient.password == "" {
			g.logger.Info("ntrip_password set to empty")
		}
		g.ntripClient.writepath = config.Attributes.String(ntripPathAttrName)
		if g.ntripClient.writepath == "" {
			g.logger.Info("ntrip_path will use same path for writing RCTM messages to gps")
			g.ntripClient.writepath = serialPath
		}
		g.ntripClient.wbaud = config.Attributes.Int(ntripBaudAttrName, 38400)
		if g.ntripClient.wbaud == 38400 {
			g.logger.Info("ntrip_baud using default baud rate 38400")
		}
		g.ntripClient.sendNMEA = config.Attributes.Bool(ntripSendNmeaName, false)
		if !g.ntripClient.sendNMEA {
			g.logger.Info("ntrip_send_nmea set to false")
		}
		g.startNtripClientRequest()
		g.Start()
	} else {
		g.ntripClient.sendNMEA = false
		g.Start()
	}

	return g, nil
}

func (g *serialNMEAGPS) fetchNtripAndUpdate() error {
	// setup the ntrip client and write the RTCM data stream to the gps
	// talk to the gps network, looking for mount points
	g.ntripClient.nmeaR, g.ntripClient.nmeaW = io.Pipe()
	req, err := ntrip.NewClientRequest(g.ntripClient.url)
	if err != nil {
		return err
	}
	req.SetBasicAuth(g.ntripClient.username, g.ntripClient.password)
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

	reqPost, err := ntrip.NewServerRequest(g.ntripClient.url, g.ntripClient.nmeaR)
	if err != nil {
		return multierr.Combine(err, resp.Body.Close())
	}
	reqPost = reqPost.WithContext(g.cancelCtx)

	respPost, err := http.DefaultClient.Do(reqPost)
	if err != nil {
		err = utils.FilterOutError(err, context.Canceled)
		if err == nil {
			g.logger.Debug("error send ntrip request: context cancelled")
		}
		return multierr.Combine(err, resp.Body.Close())
	} else if respPost.StatusCode != http.StatusOK {
		return multierr.Combine(err, errors.New("received non-200 response code: "+strconv.Itoa(respPost.StatusCode)), resp.Body.Close())
	}

	// setup port to write to
	options := slib.OpenOptions{
		BaudRate:        uint(g.ntripClient.wbaud),
		DataBits:        8,
		StopBits:        1,
		MinimumReadSize: 1,
	}
	options.PortName = g.ntripClient.writepath
	port, err := slib.Open(options)
	if err != nil {
		return multierr.Combine(err, resp.Body.Close(), respPost.Body.Close())
	}
	w := bufio.NewWriter(port)

	// Read from resp.Body until EOF
	r := io.TeeReader(resp.Body, w)
	_, err = io.ReadAll(r)
	if err != nil {
		return multierr.Combine(err, resp.Body.Close(), respPost.Body.Close())
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
				err := g.ntripClient.nmeaW.Close()
				if err != nil {
					g.logger.Debugf("can't close ntrip writer %s", err)
				}
				return
			default:
			}

			line, err := r.ReadString('\n')
			if err != nil {
				g.logger.Fatalf("can't read gps serial %s", err)
			}
			// Update our struct's gps data in-place
			g.mu.Lock()
			err = g.data.parseAndUpdate(line, g.ntripClient)
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

// Do is unimplemented.
func (g *serialNMEAGPS) Do(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	return nil, errors.New("Do() unimplemented")
}

// toPoint converts a nmea.GLL to a geo.Point.
func toPoint(a nmea.GLL) *geo.Point {
	return geo.NewPoint(a.Latitude, a.Longitude)
}
