package nmea

import (
	// "fmt"
	"context"
	"io"
	"bufio"

	"github.com/edaniels/golog"
	geo "github.com/kellydunn/golang-geo"
	"github.com/de-bkg/gognss/pkg/ntrip"
	"github.com/go-gnss/rtcm/rtcm3"
	slib "github.com/jacobsa/go-serial/serial"

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

type RTKGPS struct {
	generic.Unimplemented
	nmeagps					gps.NMEAGPS
	ntripInputProtocol		string
	ntripClient				ntripInfo
	logger 					golog.Logger
	port 					*bufio.Writer
	ntripStatus				bool
}

type ntripInfo struct {
	url       			string
	username  			string
	password  			string
	mountPoint			string
	writepath 			string
	wbaud     			int
	sendNMEA  			bool
	nmeaR     			*io.PipeReader
	nmeaW     			*io.PipeWriter
	client	  			*ntrip.Client
	stream				io.ReadCloser
	maxConnectAttempts	int
}

// Communication protocols that may be used to send NTRIP correction data
var inputProtocols = map[string]bool{
	"serial": true,
	// "I2C": true, // uncomment this one we implement I2C
}

const (
	ntripAddrAttrName = "ntrip_addr"
	ntripUserAttrName = "ntrip_username"
	ntripPassAttrName = "ntrip_password"
	ntripMountPointAttrName = "ntrip_mountpoint"
	ntripPathAttrName = "ntrip_path"
	ntripBaudAttrName = "ntrip_baud"
	ntripSendNmeaName = "ntrip_send_nmea"
	ntripInputProtocolAttrName = "ntrip_input_protocol"
	ntripConnectAttemptsName = "ntrip_connect_attempts"
)

func newRTKGPS(ctx context.Context, config config.Component, logger golog.Logger) (gps.NMEAGPS, error) {
	g := &RTKGPS{}

	g.ntripInputProtocol = config.Attributes.String(ntripInputProtocolAttrName)

	// Init NMEAGPS
	switch g.ntripInputProtocol {
	case "serial":
		var err error
		g.nmeagps, err = newSerialNMEAGPS(ctx, config, logger)
		if err != nil {
			return nil, err
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

	g.Start(ctx)

	return g, nil
}

func (g *RTKGPS) Start(ctx context.Context) {
	switch g.ntripInputProtocol {
	case "serial":
		go g.ReceiveAndWriteSerial()
	}

	g.nmeagps.Start(ctx)
}

// attempts to connect to ntrip client until successful connection or timeout
func (g *RTKGPS) Connect(casterAddr string, user string, pwd string, maxAttempts int) (*ntrip.Client, error) {
	success := false
	attempts := 0

	var c *ntrip.Client
	var err error

	g.logger.Info("Connecting")
	for !success && attempts < maxAttempts {
		g.logger.Info("...")
		c, err = ntrip.NewClient(casterAddr, ntrip.Options{Username: user, Password: pwd})
		if err == nil {
			success = true
		}
		attempts++
	}
	g.logger.Info("\n")

	if err != nil {
		g.logger.Fatalf("Can't connect to NTRIP caster: %s", err)
	}

	g.ntripClient.client = c

	return c, err
}

// attempts to connect to ntrip streak until successful connection or timeout
func (g *RTKGPS) GetStream(mountPoint string, maxAttempts int) (io.ReadCloser, error) {
	success := false
	attempts := 0

	var rc io.ReadCloser
	var err error

	g.logger.Info("Getting Stream")

	for !success && attempts < maxAttempts {
		g.logger.Info(("..."))
		rc, err = g.ntripClient.client.GetStream(mountPoint)
		if err == nil {
			success = true
		}
		attempts++
	}
	g.logger.Info("\n")

	if err != nil {
		g.logger.Fatalf("Can't connect to NTRIP stream: %s", err)
	}

	g.ntripClient.stream = rc

	return rc, err
}

func (g *RTKGPS) ReceiveAndWriteSerial() {
	c, err := g.Connect(g.ntripClient.url, g.ntripClient.username, g.ntripClient.password, g.ntripClient.maxConnectAttempts)
	if err != nil {
		g.logger.Debug(err)
	}
	defer c.CloseIdleConnections()

    if !c.IsCasterAlive() {
        g.logger.Infof("caster %s seems to be down", g.ntripClient.url)
    }

	options := slib.OpenOptions{
		PortName: g.ntripClient.writepath,
		BaudRate: uint(g.ntripClient.wbaud),
		DataBits: 8,
		StopBits: 1,
		MinimumReadSize: 1,
	}

	// Open the port.
	port, err := slib.Open(options)
	if err != nil {
		g.logger.Fatalf("serial.Open: %v", err)
	}
	defer port.Close()

	g.port = bufio.NewWriter(port)
	
	rc, err := g.GetStream(g.ntripClient.mountPoint, g.ntripClient.maxConnectAttempts)
	if err != nil {
		g.logger.Debug(err)
	}
	if rc != nil {
		defer rc.Close()
	}
	
	r := io.TeeReader(rc, g.port)
	scanner := rtcm3.NewScanner(r)

	g.ntripStatus = true

	//reads in messages while stream is connected 
	for err == nil {
		msg, err := scanner.NextMessage()
		if err != nil {
			//checks to make sure valid rtcm message has been received
			g.ntripStatus = false
			if msg == nil {
				g.logger.Info("No message... reconnecting to stream...")
				rc, err = g.GetStream(g.ntripClient.mountPoint, g.ntripClient.maxConnectAttempts)
				defer rc.Close()

				r = io.TeeReader(rc, g.port)
				scanner = rtcm3.NewScanner(r)
				g.ntripStatus = true
				continue
			} 
		}
	}
}

func (g *RTKGPS) NtripStatus() (bool, error) {
	// returns true if connection to NTRIP stream is OK, false if not
	return g.ntripStatus, nil
}

func (g *RTKGPS) ReadLocation(ctx context.Context) (*geo.Point, error) {
	return g.nmeagps.ReadLocation(ctx)
}

func (g *RTKGPS) ReadAltitude(ctx context.Context) (float64, error) {
	return g.nmeagps.ReadAltitude(ctx)
}

func (g *RTKGPS) ReadSpeed(ctx context.Context) (float64, error) {
	return g.nmeagps.ReadSpeed(ctx)
}

func (g *RTKGPS) ReadSatellites(ctx context.Context) (int, int, error) {
	return g.nmeagps.ReadSatellites(ctx)
}

func (g *RTKGPS) ReadAccuracy(ctx context.Context) (float64, float64, error) {
	return g.nmeagps.ReadAccuracy(ctx)
}

func (g *RTKGPS) ReadValid(ctx context.Context) (bool, error) {
	return g.nmeagps.ReadValid(ctx)
}

func (g *RTKGPS) Close() error {
	// close NMEAGPS
	g.nmeagps.Close()

	// TODO: close any ntrip connections

	return nil
}