package rtk

import (
	"context"
	"fmt"
	"io"
	"sync"
	"errors"

	"github.com/edaniels/golog"
	geo "github.com/kellydunn/golang-geo"
	"go.viam.com/utils/serial"

	"go.viam.com/rdk/component/board"
	"go.viam.com/rdk/component/generic"
	"go.viam.com/rdk/component/gps"
	"go.viam.com/rdk/component/gps/nmea"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/registry"
	rdkutils "go.viam.com/rdk/utils"
	"go.viam.com/utils"
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
	serialPorts				[]io.Writer
	serialWriter			io.Writer
	gpsNames				[]string

	cancelCtx               context.Context
	cancelFunc              func()
	activeBackgroundWorkers sync.WaitGroup
}

type correctionSource interface {
	GetReader() (io.ReadCloser, error)
	Start(ready chan<- bool)
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

func newRTKStation(ctx context.Context, deps registry.Dependencies, config config.Component, logger golog.Logger) (gps.LocalGPS, error) {
	cancelCtx, cancelFunc := context.WithCancel(ctx)

	r := &RTKStation{cancelCtx: cancelCtx, cancelFunc: cancelFunc, logger: logger}

	r.correctionType = config.Attributes.String(correctionSourceName)

	// Init correction source
	var err error
	switch r.correctionType {
	case "ntrip":
		r.correction, err = newNtripCorrectionSource(ctx, config, logger)
		if err != nil {
			return nil, err
		}
	case "serial":
		r.correction, err = newSerialCorrectionSource(ctx, config, logger)
		if err != nil {
			return nil, err
		}
	case "I2C":
		r.correction, err = newI2CCorrectionSource(ctx, deps, config, logger)
		if err != nil {
			return nil, err
		}
	default:
		// Invalid source
		return nil, fmt.Errorf("%s is not a valid correction source", r.correctionType)
	}

	r.gpsNames = config.Attributes.StringSlice(childrenName)

	// Init gps correction input addresses
	r.logger.Debug("Init gps")
	r.serialPorts = make([]io.Writer, 0)
	for _, gpsName := range r.gpsNames {
		gps, err := gps.FromDependencies(deps, gpsName)
		localgps := rdkutils.UnwrapProxy(gps)
		if err != nil {
			return nil, err
		}

		switch localgps.(type) {
		case *nmea.SerialNMEAGPS:
			serialgps := localgps.(*nmea.SerialNMEAGPS)
			port, err := serial.Open(serialgps.GetCorrectionPath())
			if err != nil {
				return nil, err
			}

			r.serialPorts = append(r.serialPorts, port)
		case *nmea.PmtkI2CNMEAGPS:
			i2cgps := localgps.(*nmea.PmtkI2CNMEAGPS)
			bus, addr := i2cgps.GetBusAddr()
			busAddr := i2cBusAddr{bus: bus, addr: addr}

			r.i2cPaths = append(r.i2cPaths, busAddr)
		default:
			return nil, errors.New("Child is not valid nmeaGPS type")
		}
	}

	r.logger.Debug("Init multiwriter")
	r.serialWriter = io.MultiWriter(r.serialPorts...)
	r.logger.Debug("Starting")

	r.Start(ctx)
	return r, nil
}

// Start starts reading from the correction source and sends corrections to the child gps's
func (r *RTKStation) Start(ctx context.Context) {
	r.activeBackgroundWorkers.Add(1)
	utils.PanicCapturingGo(func() {
		defer r.activeBackgroundWorkers.Done()

		// read from correction source
		ready := make(chan bool)
		go r.correction.Start(ready)

		<-ready
		stream, err := r.correction.GetReader()
		if err != nil {
			r.logger.Fatalf("Unable to get reader: %s", err)
		}

		reader := io.TeeReader(stream, r.serialWriter)

		if r.correctionType == "ntrip" {
			r.correction.(*ntripCorrectionSource).ntripStatus = true
		}

		// write corrections to all open ports and i2c handles
		for {
			select {
			case <-r.cancelCtx.Done():
				return
			default:
			}

			buf := make([]byte, 1100)
			_, err := reader.Read(buf)
			if err != nil {
				if err.Error() == "io: read/write on closed pipe" {
					r.logger.Debug("Pipe closed")
					return
				}
				r.logger.Fatalf("Unable to read stream: %s", err)
			}
			
			// write buf to all i2c handles
			for _, busAddr := range r.i2cPaths {
				//open handle
				handle, err := busAddr.bus.OpenHandle(busAddr.addr)
				if err != nil {
					r.logger.Fatalf("can't open gps i2c handle: %s", err)
					return
				}
				//write to i2c handle
				err = handle.Write(ctx, buf)
				if err != nil {
					r.logger.Fatalf("i2c handle write failed %s", err)
					return
				}
				//close i2c handle
				err = handle.Close()
                if err != nil {
                    r.logger.Fatalf("failed to close handle: %s", err)
                    return
                }
            }
        }
    })
}

// Close shuts down the RTKStation
func (r *RTKStation) Close() error {
	r.logger.Debug("Closing RTK Station")
	// close correction source
	err := r.correction.Close()
	if err != nil {
		return err
	}

	r.cancelFunc()
	r.activeBackgroundWorkers.Wait()

	// close all ports in slice
	for _, port := range r.serialPorts {
		port.(io.ReadWriteCloser).Close()
	}

	r.logger.Debug("Closing RTK Station Closed")
	return nil
}

// ReadLocation implements a LocalGPS function, but returns nil since the RTKStation does not have GPS data
func (g *RTKStation) ReadLocation(ctx context.Context) (*geo.Point, error) {
	return nil, nil
}

// ReadAltitude implements a LocalGPS function, but returns 0 since the RTKStation does not have GPS data
func (g *RTKStation) ReadAltitude(ctx context.Context) (float64, error) {
	return 0, nil
}

// ReadSpeed implements a LocalGPS function, but returns 0 since the RTKStation does not have GPS data
func (g *RTKStation) ReadSpeed(ctx context.Context) (float64, error) {
	return 0, nil
}

// ReadSatellites implements a LocalGPS function, but returns 0, 0 since the RTKStation does not have GPS data
func (g *RTKStation) ReadSatellites(ctx context.Context) (int, int, error) {
	return 0, 0, nil
}

// ReadAccuracy implements a LocalGPS function, but returns 0, 0 since the RTKStation does not have GPS data
func (g *RTKStation) ReadAccuracy(ctx context.Context) (float64, float64, error) {
	return 0, 0, nil
}

// ReadAcReadValidcuracy implements a LocalGPS function, but returns false since the RTKStation does not have GPS data
func (g *RTKStation) ReadValid(ctx context.Context) (bool, error) {
	return false, nil
}