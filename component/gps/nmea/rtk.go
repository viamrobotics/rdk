package nmea

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"sync"
	"bytes"

	"github.com/de-bkg/gognss/pkg/ntrip"
	"github.com/edaniels/golog"
	"github.com/go-gnss/rtcm/rtcm3"
	slib "github.com/jacobsa/go-serial/serial"
	geo "github.com/kellydunn/golang-geo"

	"go.viam.com/rdk/component/generic"
	"go.viam.com/rdk/component/board"
	"go.viam.com/rdk/component/gps"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/registry"
)

func init() {
	registry.RegisterComponent(
		gps.Subtype,
		"rtk",
		registry.Component{Constructor: func(
			ctx context.Context,
			deps registry.Dependencies,
			config config.Component,
			logger golog.Logger,
		) (interface{}, error) {
			return newRTKGPS(ctx, deps, config, logger)
		}})
}

// A nmeaGPS represents a GPS that can read and parse NMEA messages.
type nmeaGPS interface {
	gps.LocalGPS
	Start(ctx context.Context)                // Initialize and run GPS
	Close() error                             // Close GPS
	ReadFix(ctx context.Context) (int, error) // Returns the fix quality of the current GPS measurements
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

	bus    board.I2C
	addr   byte

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

func newRTKGPS(ctx context.Context, deps registry.Dependencies, config config.Component, logger golog.Logger) (nmeaGPS, error) {
	cancelCtx, cancelFunc := context.WithCancel(ctx)

	g := &RTKGPS{cancelCtx: cancelCtx, cancelFunc: cancelFunc, logger: logger}

	g.ntripInputProtocol = config.Attributes.String(ntripInputProtocolAttrName)

	// Init NMEAGPS
	switch g.ntripInputProtocol {
	case "serial":
		var err error
		g.nmeagps, err = newSerialNMEAGPS(ctx, config, logger)
		if err != nil {
			return nil, err
		}
	case "I2C":
		var err error
		g.nmeagps, err = newPmtkI2CNMEAGPS(ctx, deps, config, logger)
		if err != nil {
			return nil, err
		}
		b, err := board.FromDependencies(deps, config.Attributes.String("board"))
		if err != nil {
			return nil, fmt.Errorf("gps init: failed to find board: %w", err)
		}
		localB, ok := b.(board.LocalBoard)
		if !ok {
			return nil, fmt.Errorf("board %s is not local", config.Attributes.String("board"))
		}
		i2cbus, ok := localB.I2CByName(config.Attributes.String("bus"))
		if !ok {
			return nil, fmt.Errorf("gps init: failed to find i2c bus %s", config.Attributes.String("bus"))
		} else {
			g.bus = i2cbus
		}

		addr := config.Attributes.Int("i2c_addr", -1)
		if addr == -1 {
			return nil, errors.New("must specify gps i2c address")
		} else {
			g.addr = byte(addr)
		}
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
	g.logger.Info("Starting rtk START")
	g.Start(ctx)

	return g, nil
}

// Start begins NTRIP receiver with specified protocol and begins reading/updating GPS measurements.
func (g *RTKGPS) Start(ctx context.Context) {
	switch g.ntripInputProtocol {
	case "serial":
		go g.ReceiveAndWriteSerial()
	case "I2C":
		g.ReceiveAndWriteI2C(ctx)
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

func (g *RTKGPS) ReceiveAndWriteI2C(ctx context.Context) {
	g.activeBackgroundWorkers.Add(1)
	defer g.activeBackgroundWorkers.Done()
	err := g.Connect(g.ntripClient.url, g.ntripClient.username, g.ntripClient.password, g.ntripClient.maxConnectAttempts)
	if err != nil {
		return
	}

	if !g.ntripClient.client.IsCasterAlive() {
		g.logger.Infof("caster %s seems to be down", g.ntripClient.url)
	}

	//establish I2C connection
	handle, err := g.bus.OpenHandle(g.addr)
	if err != nil {
		g.logger.Fatalf("can't open gps i2c %s", err)
		return
	}
	// Send GLL, RMC, VTG, GGA, GSA, and GSV sentences each 1000ms
	cmd251 := addChk([]byte("PMTK251,115200")) //set baud rate
	cmd314 := addChk([]byte("PMTK314,1,1,1,1,1,0,0,0,0,0,0,0,0,0,0,0,0,0,0"))
	cmd220 := addChk([]byte("PMTK220,1000"))

	err = handle.Write(ctx, cmd251)
	if err != nil {
		g.logger.Debug("Failed to set baud rate")
	}
	err = handle.Write(ctx, cmd314)
	if err != nil {
		g.logger.Debug("failed to set NMEA output")
		return
	}
	err = handle.Write(ctx, cmd220)
	if err != nil {
		g.logger.Debug("failed to set NMEA update rate")
		return
	}

	
	err = g.GetStream(g.ntripClient.mountPoint, g.ntripClient.maxConnectAttempts)
	if err != nil {
		return
	}

	// create a buffer
	w := &bytes.Buffer{}
	r := io.TeeReader(g.ntripClient.stream, w)
	
	buf := make([]byte, 1100)
	n, err := g.ntripClient.stream.Read(buf)
	w_i2c := addChk(buf[:n])

	//port still open
	// g.logger.Infof("writing: %s", w_i2c)
	err = handle.Write(ctx, w_i2c)
	if err != nil {
		g.logger.Fatalf("uh oh, i2c handle write failed %s", err)
		return
	}

	scanner := rtcm3.NewScanner(r)

	for err == nil {
		msg, err := scanner.NextMessage()
		//establish I2C connection
		handle, err := g.bus.OpenHandle(g.addr)
		if err != nil {
			g.logger.Fatalf("can't open gps i2c %s", err)
			return
		}
		if err != nil {
			if msg == nil {
				fmt.Println("No message... reconnecting to stream...")
				err = g.GetStream(g.ntripClient.mountPoint, g.ntripClient.maxConnectAttempts)
				if err != nil {
					return
				}

				w = &bytes.Buffer{}
				r = io.TeeReader(g.ntripClient.stream, w)
				
				buf = make([]byte, 1100)
				n, err := g.ntripClient.stream.Read(buf)
				w_i2c := addChk(buf[:n])
							
				err = handle.Write(ctx, w_i2c)

				if err != nil {
					g.logger.Debug("i2c handle write failed %s", err)
					return
				}

				scanner = rtcm3.NewScanner(r)
				continue
			}
			g.logger.Fatal(err, msg)
		}
		err = handle.Close()
		if err != nil {
			g.logger.Debug("failed to close handle: %s", err)
			return
		}
	}
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

// ReadFix returns Fix quality of GPS measurements.
func (g *RTKGPS) ReadFix(ctx context.Context) (int, error) {
	return g.nmeagps.ReadFix(ctx)
}

// GetReadings will use the default GPS GetReadings if not provided.
func (g *RTKGPS) GetReadings(ctx context.Context) ([]interface{}, error) {
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
