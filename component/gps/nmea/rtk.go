package nmea

import (
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
	lg	gps.LocalGPS
	correctionInputProtocol	string
	n	ntripInfo
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
	// Communication protocols that may be used to send NTRIP correction data
	serialName = "serial"
	i2CName	= "I2C"
)

func newRTKGPS(ctx context.Context, config config.Component, logger golog.Logger) (gps.LocalGPS, error) {
	// TODO
	g := &RTKGPS{}
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
	// TODO: close localGPS (may have to make a new interface to include their individual Close funcs)

	// TODO: close any ntrip connections if neccessary

	return nil
}