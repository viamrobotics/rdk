package nmea

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"sync"

	"github.com/de-bkg/gognss/pkg/ntrip"
	"github.com/edaniels/golog"
	"github.com/go-gnss/rtcm/rtcm3"
	slib "github.com/jacobsa/go-serial/serial"
	geo "github.com/kellydunn/golang-geo"

	"go.viam.com/rdk/component/generic"
	"go.viam.com/rdk/component/gps"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/robot"
)

func init() {
	registry.RegisterComponent(
		gps.Subtype,
		"rtk",
		registry.Component{Constructor: func(
			ctx context.Context,
			r robot.Robot,
			config config.Component,
			logger golog.Logger,
		) (interface{}, error) {
			return newRTKGPS(ctx, config, logger)
		}})
}

// A NMEAGPS represents a GPS that can read and parse NMEA messages.
type nmeaGPS interface {
	gps.LocalGPS
	Start(ctx context.Context) // Initialize and run GPS
	Close() error              // Close GPS
}

// A RTKGPS is an NMEA GPS model that can intake RTK correction data.
type RTKGPS struct {
	generic.Unimplemented
	nmeagps            nmeaGPS
	ntripInputProtocol string
	ntripClient        ntripInfo
	logger             golog.Logger
	correctionWriter   io.ReadWriteCloser
	ntripStatus        bool

	cancelCtx               context.Context
	cancelFunc              func()
	activeBackgroundWorkers sync.WaitGroup
}

type ntripInfo struct {
	url                string
	username           string
	password           string
	mountPoint         string
	writepath          string
	wbaud              int
	sendNMEA           bool
	client             *ntrip.Client
	stream             io.ReadCloser
	maxConnectAttempts int
}

const (
	ntripAddrAttrName          = "ntrip_addr"
	ntripUserAttrName          = "ntrip_username"
	ntripPassAttrName          = "ntrip_password"
	ntripMountPointAttrName    = "ntrip_mountpoint"
	ntripPathAttrName          = "ntrip_path"
	ntripBaudAttrName          = "ntrip_baud"
	ntripSendNmeaName          = "ntrip_send_nmea"
	ntripInputProtocolAttrName = "ntrip_input_protocol"
	ntripConnectAttemptsName   = "ntrip_connect_attempts"
)

func newRTKGPS(ctx context.Context, config config.Component, logger golog.Logger) (nmeaGPS, error) {
	cancelCtx, cancelFunc := context.WithCancel(ctx)

	g := &RTKGPS{cancelCtx: cancelCtx, cancelFunc: cancelFunc, logger: logger}

	g.ntripInputProtocol = config.Attributes.String(ntripInputProtocolAttrName)

	// Init NMEAGPS
	switch g.ntripInputProtocol {
	case "serial":
		var err error
		localgps, err := newSerialNMEAGPS(ctx, config, logger)
		if err != nil {
			return nil, err
		}
		g.nmeagps = localgps.(nmeaGPS)
	case "I2C":
		return nil, errors.New("I2C not implemented")
	default:
		// Invalid protocol
		return nil, fmt.Errorf("%s is not a valid protocol", g.ntripInputProtocol)
	}

	// Init ntripInfo from attributes
	g.ntripClient.url = config.Attributes.String(ntripAddrAttrName)
	if g.ntripClient.url == "" {
		return nil, fmt.Errorf("RTKGPS expected non-empty string for %q", ntripAddrAttrName)
	}
	g.ntripClient.username = config.Attributes.String(ntripUserAttrName)
	if g.ntripClient.username == "" {
		g.logger.Info("ntrip_username set to empty")
	}
	g.ntripClient.password = config.Attributes.String(ntripPassAttrName)
	if g.ntripClient.password == "" {
		g.logger.Info("ntrip_password set to empty")
	}
	g.ntripClient.mountPoint = config.Attributes.String(ntripMountPointAttrName)
	if g.ntripClient.mountPoint == "" {
		g.logger.Info("ntrip_mountpoint set to empty")
	}
	g.ntripClient.writepath = config.Attributes.String(ntripPathAttrName)
	if g.ntripClient.writepath == "" {
		g.logger.Info("ntrip_path will use same path for writing RCTM messages to gps")
		g.ntripClient.writepath = config.Attributes.String(pathAttrName)
	}
	g.ntripClient.wbaud = config.Attributes.Int(ntripBaudAttrName, 38400)
	if g.ntripClient.wbaud == 38400 {
		g.logger.Info("ntrip_baud using default baud rate 38400")
	}
	g.ntripClient.sendNMEA = config.Attributes.Bool(ntripSendNmeaName, false)
	if !g.ntripClient.sendNMEA {
		g.logger.Info("ntrip_send_nmea set to false")
	}
	g.ntripClient.maxConnectAttempts = config.Attributes.Int(ntripConnectAttemptsName, 10)
	if g.ntripClient.maxConnectAttempts == 10 {
		g.logger.Info("ntrip_connect_attempts using default 10")
	}

	g.Start(ctx)

	return g, nil
}

// Start begins NTRIP receiver with specified protocol and begins reading/updating GPS measurements.
func (g *RTKGPS) Start(ctx context.Context) {
	switch g.ntripInputProtocol {
	case "serial":
		go g.ReceiveAndWriteSerial()
	case "I2C":
		g.logger.Error("I2C not implemented")
	}

	g.nmeagps.Start(ctx)
}

// Connect attempts to connect to ntrip client until successful connection or timeout.
func (g *RTKGPS) Connect(casterAddr string, user string, pwd string, maxAttempts int) error {
	success := false
	attempts := 0

	var c *ntrip.Client
	var err error

	g.logger.Debug("Connecting to NTRIP caster")
	for !success && attempts < maxAttempts {
		select {
		case <-g.cancelCtx.Done():
			return errors.New("Canceled")
		default:
		}

		c, err = ntrip.NewClient(casterAddr, ntrip.Options{Username: user, Password: pwd})
		if err == nil {
			success = true
		}
		attempts++
	}

	if err != nil {
		g.logger.Errorf("Can't connect to NTRIP caster: %s", err)
		return err
	}

	g.ntripClient.client = c

	g.logger.Debug("Connected to NTRIP caster")

	return nil
}

// GetStream attempts to connect to ntrip streak until successful connection or timeout.
func (g *RTKGPS) GetStream(mountPoint string, maxAttempts int) error {
	success := false
	attempts := 0

	var rc io.ReadCloser
	var err error

	g.logger.Debug("Getting NTRIP stream")

	for !success && attempts < maxAttempts {
		select {
		case <-g.cancelCtx.Done():
			return errors.New("Canceled")
		default:
		}

		rc, err = g.ntripClient.client.GetStream(mountPoint)
		if err == nil {
			success = true
		}
		attempts++
	}

	if err != nil {
		g.logger.Errorf("Can't connect to NTRIP stream: %s", err)
		return err
	}

	g.ntripClient.stream = rc

	g.logger.Debug("Connected to stream")

	return nil
}

// ReceiveAndWriteSerial connects to NTRIP receiver and sends correction stream to the GPS through serial.
func (g *RTKGPS) ReceiveAndWriteSerial() {
	g.activeBackgroundWorkers.Add(1)
	defer g.activeBackgroundWorkers.Done()
	err := g.Connect(g.ntripClient.url, g.ntripClient.username, g.ntripClient.password, g.ntripClient.maxConnectAttempts)
	if err != nil {
		return
	}

	if !g.ntripClient.client.IsCasterAlive() {
		g.logger.Infof("caster %s seems to be down", g.ntripClient.url)
	}

	options := slib.OpenOptions{
		PortName:        g.ntripClient.writepath,
		BaudRate:        uint(g.ntripClient.wbaud),
		DataBits:        8,
		StopBits:        1,
		MinimumReadSize: 1,
	}

	// Open the port.
	g.correctionWriter, err = slib.Open(options)
	if err != nil {
		g.logger.Errorf("serial.Open: %v", err)
		return
	}

	w := bufio.NewWriter(g.correctionWriter)

	err = g.GetStream(g.ntripClient.mountPoint, g.ntripClient.maxConnectAttempts)
	if err != nil {
		return
	}

	r := io.TeeReader(g.ntripClient.stream, w)
	scanner := rtcm3.NewScanner(r)

	g.ntripStatus = true

	for g.ntripStatus {
		select {
		case <-g.cancelCtx.Done():
			return
		default:
		}

		msg, err := scanner.NextMessage()
		if err != nil {
			g.ntripStatus = false
			if msg == nil {
				g.logger.Debug("No message... reconnecting to stream...")
				err = g.GetStream(g.ntripClient.mountPoint, g.ntripClient.maxConnectAttempts)
				if err != nil {
					return
				}

				r = io.TeeReader(g.ntripClient.stream, w)
				scanner = rtcm3.NewScanner(r)
				g.ntripStatus = true
				continue
			}
		}
	}
}

// NtripStatus returns true if connection to NTRIP stream is OK, false if not.
func (g *RTKGPS) NtripStatus() (bool, error) {
	return g.ntripStatus, nil
}

// ReadLocation returns the current geographic location of the GPS.
func (g *RTKGPS) ReadLocation(ctx context.Context) (*geo.Point, error) {
	return g.nmeagps.ReadLocation(ctx)
}

// ReadAltitude returns the current altitude of the GPS.
func (g *RTKGPS) ReadAltitude(ctx context.Context) (float64, error) {
	return g.nmeagps.ReadAltitude(ctx)
}

// ReadSpeed returns the current speed of the GPS.
func (g *RTKGPS) ReadSpeed(ctx context.Context) (float64, error) {
	return g.nmeagps.ReadSpeed(ctx)
}

// ReadSatellites returns the number of satellites that are currently visible to the GPS.
func (g *RTKGPS) ReadSatellites(ctx context.Context) (int, int, error) {
	return g.nmeagps.ReadSatellites(ctx)
}

// ReadAccuracy returns how accurate the lat/long readings are.
func (g *RTKGPS) ReadAccuracy(ctx context.Context) (float64, float64, error) {
	return g.nmeagps.ReadAccuracy(ctx)
}

// ReadValid returns whether or not the GPS is currently reading valid measurements.
func (g *RTKGPS) ReadValid(ctx context.Context) (bool, error) {
	return g.nmeagps.ReadValid(ctx)
}

// ReadFix returns Fix quality of GPS measurements
func (g *RTKGPS) ReadFix(ctx context.Context) (int, error) {
	return g.nmeagps.ReadFix(ctx)
}

// Close shuts down the RTKGPS.
func (g *RTKGPS) Close() error {
	g.cancelFunc()
	g.activeBackgroundWorkers.Wait()

	err := g.nmeagps.Close()
	if err != nil {
		return err
	}

	// close ntrip writer
	if g.correctionWriter != nil {
		if err := g.correctionWriter.Close(); err != nil {
			return err
		}
		g.correctionWriter = nil
	}

	// close ntrip client and stream
	if g.ntripClient.client != nil {
		g.ntripClient.client.CloseIdleConnections()
		g.ntripClient.client = nil
	}

	if g.ntripClient.stream != nil {
		if err := g.ntripClient.stream.Close(); err != nil {
			return err
		}
		g.ntripClient.stream = nil
	}

	return nil
}
