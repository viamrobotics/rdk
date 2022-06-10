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
	g	gps.LocalGPS
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

func newSerialNMEAGPS(ctx context.Context, config config.Component, logger golog.Logger) (gps.LocalGPS, error) {

}

func (g *RTKGPS) ReadLocation(ctx context.Context) (*geo.Point, error) {
	return g.ReadLocation(ctx)
}

func (g *RTKGPS) ReadAltitude(ctx context.Context) (float64, error) {
	g.ReadAltitude(ctx)
}

func (g *RTKGPS) ReadSpeed(ctx context.Context) (float64, error) {
	return g.ReadSpeed(ctx)
}

func (g *RTKGPS) ReadSatellites(ctx context.Context) (int, int, error) {
	return g.ReadSatellites(ctx)
}

func (g *RTKGPS) ReadAccuracy(ctx context.Context) (float64, float64, error) {
	return g.ReadAccuracy(ctx)
}

func (g *RTKGPS) ReadValid(ctx context.Context) (bool, error) {
	return g.ReadValid(ctx)
}

func (g *RTKGPS) Close() error {
	return g.Close()

	// TODO: close any ntrip connections if neccessary
}