package nmea

import (
	"fmt"
	"context"
	"io"

	"github.com/edaniels/golog"
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
		"nmea-serial-rtk",
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
	lg	gps.NMEAGPS
	ntripInputProtocol	string
	ntripClient	ntripInfo
	logger golog.Logger
	// TODO: maybe add channel for communicated between ntrip receiver and nmea parser (i.e. when no stream can be received)
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

// Communication protocols that may be used to send NTRIP correction data
var inputProtocols = map[string]bool{
	"serial": true,
	// "I2C": true, // uncomment this one we implement I2C
}

const (
	ntripAddrAttrName = "ntrip_addr"
	ntripUserAttrName = "ntrip_username"
	ntripPassAttrName = "ntrip_password"
	ntripPathAttrName = "ntrip_path"
	ntripBaudAttrName = "ntrip_baud"
	ntripSendNmeaName = "ntrip_send_nmea"
	ntripInputProtocolAttrName = "ntrip_input_protocol"
)

func newRTKGPS(ctx context.Context, config config.Component, logger golog.Logger) (gps.LocalGPS, error) {
	g := &RTKGPS{}

	g.ntripInputProtocol = config.Attributes.String(ntripInputProtocolAttrName)

	// Init localGPS
	switch g.ntripInputProtocol {
	case "serial":
		var err error
		g.lg, err = newSerialNMEAGPS(ctx, config, logger)
		if err != nil {
			return nil, err
		}
	default:
		// Invalid protocol
		return nil, fmt.Errorf("%s is not a valid protocol", g.ntripInputProtocol)
	}
	
	// Init ntripInfo from attributes
	g.ntripClient.url = config.Attributes.String(ntripAddrAttrName)
	if g.ntripClient.url != "" {
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

	// TODO: run go process for sending ntrip here

	g.lg.Start(ctx)

	return g, nil
}

func (g *RTKGPS) ReadLocation(ctx context.Context) (*geo.Point, error) {
	return g.lg.ReadLocation(ctx)
}

func (g *RTKGPS) ReadAltitude(ctx context.Context) (float64, error) {
	return g.lg.ReadAltitude(ctx)
}

func (g *RTKGPS) ReadSpeed(ctx context.Context) (float64, error) {
	return g.lg.ReadSpeed(ctx)
}

func (g *RTKGPS) ReadSatellites(ctx context.Context) (int, int, error) {
	return g.lg.ReadSatellites(ctx)
}

func (g *RTKGPS) ReadAccuracy(ctx context.Context) (float64, float64, error) {
	return g.lg.ReadAccuracy(ctx)
}

func (g *RTKGPS) ReadValid(ctx context.Context) (bool, error) {
	return g.lg.ReadValid(ctx)
}

func (g *RTKGPS) Close() error {
	// close localGPS
	g.lg.Close()

	// TODO: close any ntrip connections

	return nil
}