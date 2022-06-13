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

	"github.com/de-bkg/gognss/pkg/ntrip"
	serialPort "github.com/jacobsa/go-serial/serial"
	"github.com/go-gnss/rtcm/rtcm3"
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
	pathAttrName      = "path"
	casterName = "ntrip_addr"
	mountpointName = "ntrip_mountpoint"
	userName = "ntrip_username"
	passwordName = "ntrip_password"
)

func newSerialNMEAGPS(ctx context.Context, config config.Component, logger golog.Logger) (gps.NMEAGPS, error) {
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

  g.logger.Debug("STARTING")
	go g.Receive(config)
	g.Start()

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
			g.logger.Debug(g.data.location)
			g.mu.Unlock()
			if err != nil {
				g.logger.Debugf("can't parse nmea %s : %s", line, err)
			}
		}
	})
}

func (g *serialNMEAGPS) Connect(casterAddr string, user string, pwd string) (*ntrip.Client, error){
	success := false

	var err error
	var c *ntrip.Client

	g.logger.Debug("Connecting")
	for !success {
		g.logger.Debug("...")
		c, err = ntrip.NewClient(casterAddr, ntrip.Options{Username: user, Password: pwd})
		if err == nil {
			success = true
			g.logger.Debug("Connection success")
		}
	}

	return c, err
	
}

func (g *serialNMEAGPS) GetStream(c *ntrip.Client, mountPoint string, maxAttempts int) (io.ReadCloser, error){
	fail := true
	attempts := 0

	var rc io.ReadCloser
	var err error

	g.logger.Debug("Getting Stream")

	for fail && attempts < maxAttempts {
		rc, err = c.GetStream(mountPoint)
		if err != nil {
			fail = false
		}
		attempts++
	}

	if err != nil {
		g.logger.Debug(err)
	}

	return rc, err
}

func (g *serialNMEAGPS) Receive(config config.Component) {
	casterAddr := config.Attributes.String(casterName)
	mountPoint := config.Attributes.String(mountpointName)
	user := config.Attributes.String(userName)
	pwd := config.Attributes.String(passwordName)

	c, err := g.Connect(casterAddr, user, pwd)
	if err!=nil {
		g.logger.Debug(err)
	}
	defer c.CloseIdleConnections()

    if !c.IsCasterAlive() {
        g.logger.Infof("caster %s seems to be down", casterAddr)
    }

	options := serialPort.OpenOptions{
		PortName: "/dev/serial/by-id/usb-u-blox_AG_-_www.u-blox.com_u-blox_GNSS_receiver-if00",
		BaudRate: 38400,
		DataBits: 8,
		StopBits: 1,
		MinimumReadSize: 1,
	}

	//Open the port.
	port, err := serialPort.Open(options)
	if err != nil {
		g.logger.Debug("serial.Open: %v", err)
	}
	
	defer port.Close()
	w := bufio.NewWriter(port)
	
	rc, err := g.GetStream(c, mountPoint, 10)
	if err != nil {
		g.logger.Debug(err)
	}
	if rc != nil {
		defer rc.Close()
	}
	
	r := io.TeeReader(rc, w)
	scanner := rtcm3.NewScanner(r)
	g.logger.Debug(scanner)

	//reads in messages while stream is connected 
	for err == nil {
		msg, err := scanner.NextMessage()
		if err != nil {
			//checks to make sure valid rtcm message has been received
			if msg == nil {
				g.logger.Info("No message... reconnecting to stream...")
				rc, err = g.GetStream(c, mountPoint, 10)
				defer rc.Close()

				r = io.TeeReader(rc, w)
				scanner = rtcm3.NewScanner(r)
				continue
			} 
		}
	}
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
