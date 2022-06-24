package rtk

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

	"go.viam.com/rdk/component/board"
	"go.viam.com/rdk/component/generic"
	"go.viam.com/rdk/component/gps"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/registry"
)

func init() {
	registry.RegisterComponent(
		gps.Subtype,
		"rtk-station",
		registry.Component{Constructor: func(
			ctx context.Context,
			_ registry.Dependencies,
			config config.Component,
			logger golog.Logger,
		) (interface{}, error) {
			return newRTKStation(ctx, config, logger)
		}})
}

type RTKStation struct {
	generic.Unimplemented
	mu     					sync.RWMutex
	logger 					golog.Logger
	correction				correctionSource
	i2cPaths				[]i2cBusAddr
	serialPaths				[]io.ReadWriteCloser

	cancelCtx               context.Context
	cancelFunc              func()
	activeBackgroundWorkers sync.WaitGroup
}

type correctionSource interface {
	GetReader() (io.ReadWriteCloser, error)
	Start(ctx context.Context)
	Close() error
}

type i2cBusAddr struct {
	bus		board.I2C
	addr	byte
}

const (
	correctionSourceName		= "correction_source"
)

func newRTKStation(ctx context.Context, config config.Component, logger golog.Logger) (RTKStation, error) {
	cancelCtx, cancelFunc := context.WithCancel(ctx)

	r := &RTKStation{cancelCtx: cancelCtx, cancelFunc: cancelFunc, logger: logger}

	correctionType := config.Attributes.String(correctionSourceName)

	// Init correction source
	switch correctionType {
	case "ntrip":
		r.correction, err := newNtripCorrectionSource(ctx, config, logger)
		if err != nil {
			return nil, err
		}
	case "serial":
		return nil, errors.New("Serial not implemented")
	case "I2C":
		return nil, errors.New("I2C not implemented")
	default:
		// Invalid source
		return nil, fmt.Errorf("%s is not a valid correction source", correctionSource)
	}

	// Init gps correction input addresses
	// TODO: Get all gps's dependent on this rtk station and check that they have either serial path or i2c bus/addr
	// TODO: open all ports and to serial slice
	// TODO: create all bus/addr structs and add to i2c slice
}

func (r *RTKStation) Start(ctx context.Context) {
	// read from correction source
	// write corrections to all open ports and i2c handles
}

func (r *RTKStation) Close() error {
	r.cancelFunc()
	r.activeBackgroundWorkers.Wait()

	// close correction source
	err := r.correction.Close()
	if err != nil {
		return err
	}

	// close all ports in slice

	return nil
}

// These are all necessary for this to be a gps... not sure of a better option right now
func (g *serialNMEAGPS) ReadLocation(ctx context.Context) (*geo.Point, error) {
	return nil, nil
}

func (g *serialNMEAGPS) ReadAltitude(ctx context.Context) (float64, error) {
	return nil, nil
}

func (g *serialNMEAGPS) ReadSpeed(ctx context.Context) (float64, error) {
	return nil, nil
}

func (g *serialNMEAGPS) ReadSatellites(ctx context.Context) (int, int, error) {
	return nil, nil
}

func (g *serialNMEAGPS) ReadAccuracy(ctx context.Context) (float64, float64, error) {
	return nil, nil, nil
}

func (g *serialNMEAGPS) ReadValid(ctx context.Context) (bool, error) {
	return nil, nil
}