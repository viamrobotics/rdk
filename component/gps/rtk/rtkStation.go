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
	"go.viam.com/rdk/component/nmea"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/registry"
)

func init() {
	registry.RegisterComponent(
		gps.Subtype,
		"rtk-station",
		registry.Component{Constructor: func(
			ctx context.Context,
			deps registry.Dependencies,
			config config.Component,
			logger golog.Logger,
		) (interface{}, error) {
			return newRTKStation(ctx, deps, config, logger)
		}})
}

type RTKStation struct {
	generic.Unimplemented
	mu     					sync.RWMutex
	logger 					golog.Logger
	correction				correctionSource
	correctionType			string
	i2cPaths				[]i2cBusAddr
	serialPorts				[]io.ReadWriteCloser
	serialWriter			io.ReadWriteCloser
	gpsNames				[]string

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
	childrenName				= "children"
)

func newRTKStation(ctx context.Context, deps registry.Dependencies, config config.Component, logger golog.Logger) (RTKStation, error) {
	cancelCtx, cancelFunc := context.WithCancel(ctx)

	r := &RTKStation{cancelCtx: cancelCtx, cancelFunc: cancelFunc, logger: logger}

	r.correctionType = config.Attributes.String(correctionSourceName)

	// Init correction source
	switch r.correctionType {
	case "ntrip":
		r.correction, err := newNtripCorrectionSource(ctx, config, logger)
		if err != nil {
			return nil, err
		}
	case "serial":
		r.correction, err := newSerialCorrectionSource(ctx, config, logger)
		if err != nil {
			return nil, err
		}
	case "I2C":
		return nil, errors.New("I2C not implemented")
	default:
		// Invalid source
		return nil, fmt.Errorf("%s is not a valid correction source", correctionSource)
	}

	r.gpsNames = config.Attributes.StringSlice(childrenName)

	// Init gps correction input addresses
	r.serialPorts = make([]io.ReadWriteCloser, 0)
	for _, gpsName := range r.gpsNames {
		gps, err := gps.FromDependencies(deps, gpsName)
		if err != nil {
			return nil, err
		}

		switch gps.(type) {
		case nmea.SerialNMEAGPS:
			gps = gps.(nmea.SerialNMEAGPS)
			port, err := serial.Open(gps.GetCorrectionPath())

			r.serialPorts = append(r.serialPorts, port)
		case nmea.PmtkI2CNMEAGPS:
			gps = gps.(nmea.PmtkI2CNMEAGPS)
			bus, addr := gps.GetBusAddr()
			busAddr := i2cBusAddr{bus: bus, addr: addr}

			r.i2cPaths = append(r.i2cPaths, busAddr)
		default:
			return nil, errors.New("Child is not valid nmeaGPS type")
		}
	}

	r.serialWriter = io.MultiWriter(r.serialPorts...)

	r.Start(ctx)
}

func (r *RTKStation) Start(ctx context.Context) {
	// read from correction source
    ready := make(chan bool, false)
	go r.correction.Start(ctx, ready)

	<-ready
	stream, err := r.GetReader()
	reader := io.TeeReader(stream, w)

	if r.correctionType == "ntrip" {
		r.correction.ntripStatus = true
	}

	// write corrections to all open ports and i2c handles
	for {
		select {
		case <-r.cancelCtx.Done():
			return
		default:
		}

		buf := make([]byte, 1100)
		n, err := reader.Read(buf)

		// write buf to all i2c handles
	}
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
	for _, port := range r.serialPorts {
		port.Close()
	}
	r.serialWriter.Close()

	// close i2c handles?

	return nil
}

// These are all necessary for this to be a gps... not sure of a better option right now
func (g *SerialNMEAGPS) ReadLocation(ctx context.Context) (*geo.Point, error) {
	return nil, nil
}

func (g *SerialNMEAGPS) ReadAltitude(ctx context.Context) (float64, error) {
	return nil, nil
}

func (g *SerialNMEAGPS) ReadSpeed(ctx context.Context) (float64, error) {
	return nil, nil
}

func (g *SerialNMEAGPS) ReadSatellites(ctx context.Context) (int, int, error) {
	return nil, nil
}

func (g *SerialNMEAGPS) ReadAccuracy(ctx context.Context) (float64, float64, error) {
	return nil, nil, nil
}

func (g *SerialNMEAGPS) ReadValid(ctx context.Context) (bool, error) {
	return nil, nil
}