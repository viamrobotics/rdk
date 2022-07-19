// Package rtk defines the rtk correction receiver
// which sends rtcm data to child gps's
package rtk

import (
	"context"
	"fmt"
	"io"
	"sync"

	"github.com/edaniels/golog"
	"github.com/jacobsa/go-serial/serial"
	geo "github.com/kellydunn/golang-geo"
	"github.com/pkg/errors"
	"go.viam.com/utils"

	"go.viam.com/rdk/component/board"
	"go.viam.com/rdk/component/generic"
	"go.viam.com/rdk/component/gps"
	"go.viam.com/rdk/component/gps/nmea"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/registry"
	rdkutils "go.viam.com/rdk/utils"
)

// AttrConfig is used for converting RTK GPS config attributes.
type AttrConfig struct {
	CorrectionSource string `json:"correction_source"`
	// ntrip
	NtripAddr            string `json:"ntrip_addr"`
	NtripConnectAttempts int    `json:"ntrip_connect_attempts,omitempty"`
	NtripMountpoint      string `json:"ntrip_mountpoint,omitempty"`
	NtripPass            string `json:"ntrip_password,omitempty"`
	NtripUser            string `json:"ntrip_username,omitempty"`
	// serial
	CorrectionPath string `json:"correction_path"`
	// I2C
	Board   string `json:"board"`
	Bus     string `json:"bus"`
	I2cAddr int    `json:"i2c_addr"`
}

// Validate ensures all parts of the config are valid.
func (config *AttrConfig) Validate(path string) error {
	if len(config.CorrectionSource) == 0 {
		return errors.New("expected nonempty correction source")
	}
	if config.CorrectionSource != serialStr && config.CorrectionSource != ntripStr && config.CorrectionSource != i2cStr {
		return errors.New("only serial, I2C, and ntrip are supported correction sources")
	}

	if config.CorrectionSource == ntripStr {
		if len(config.NtripAddr) == 0 {
			return errors.New("expected nonempty ntrip address")
		}
	}

	if config.CorrectionSource == serialStr {
		if len(config.CorrectionPath) == 0 {
			return errors.New("must specify serial path")
		}
	}

	if config.CorrectionSource == i2cStr {
		if len(config.Board) == 0 {
			return errors.New("cannot find board for rtk station")
		}
		if len(config.Bus) == 0 {
			return errors.New("cannot find i2c board for rtk station")
		}
		if config.I2cAddr <= 0 {
			return errors.New("cannot find i2c address for rtk station")
		}
	}

	return nil
}

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

type rtkStation struct {
	generic.Unimplemented
	logger         golog.Logger
	correction     correctionSource
	correctionType string
	i2cPaths       []i2cBusAddr
	serialPorts    []io.Writer
	serialWriter   io.Writer
	gpsNames       []string

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
	bus  board.I2C
	addr byte
}

const (
	correctionSourceName = "correction_source"
	childrenName         = "children"
	i2cStr               = "I2C"
	serialStr            = "serial"
	ntripStr             = "ntrip"
)

func newRTKStation(ctx context.Context, deps registry.Dependencies, config config.Component, logger golog.Logger) (gps.LocalGPS, error) {
	cancelCtx, cancelFunc := context.WithCancel(ctx)

	r := &rtkStation{cancelCtx: cancelCtx, cancelFunc: cancelFunc, logger: logger}

	r.correctionType = config.Attributes.String(correctionSourceName)

	// Init correction source
	var err error
	switch r.correctionType {
	case ntripStr:
		r.correction, err = newNtripCorrectionSource(ctx, config, logger)
		if err != nil {
			return nil, err
		}
	case serialStr:
		r.correction, err = newSerialCorrectionSource(ctx, config, logger)
		if err != nil {
			return nil, err
		}
	case i2cStr:
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

		switch t := localgps.(type) {
		case *nmea.SerialNMEAGPS:
			path, br := t.GetCorrectionInfo()

			options := serial.OpenOptions{
				PortName:        path,
				BaudRate:        br,
				DataBits:        8,
				StopBits:        1,
				MinimumReadSize: 4,
			}

			port, err := serial.Open(options)
			if err != nil {
				return nil, err
			}

			r.serialPorts = append(r.serialPorts, port)
		case *nmea.PmtkI2CNMEAGPS:
			bus, addr := t.GetBusAddr()
			busAddr := i2cBusAddr{bus: bus, addr: addr}

			r.i2cPaths = append(r.i2cPaths, busAddr)
		default:
			return nil, errors.New("child is not valid nmeaGPS type")
		}
	}

	r.logger.Debug("Init multiwriter")
	r.serialWriter = io.MultiWriter(r.serialPorts...)
	r.logger.Debug("Starting")

	r.Start(ctx)
	return r, nil
}

// Start starts reading from the correction source and sends corrections to the child gps's.
func (r *rtkStation) Start(ctx context.Context) {
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

		if r.correctionType == ntripStr {
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
			n, err := reader.Read(buf)
			r.logger.Debugf("Reading %d bytes", n)
			if err != nil {
				if err.Error() == "io: read/write on closed pipe" {
					r.logger.Debug("Pipe closed")
					return
				}
				r.logger.Fatalf("Unable to read stream: %s", err)
			}

			// write buf to all i2c handles
			for _, busAddr := range r.i2cPaths {
				// open handle
				handle, err := busAddr.bus.OpenHandle(busAddr.addr)
				if err != nil {
					r.logger.Fatalf("can't open gps i2c handle: %s", err)
					return
				}
				// write to i2c handle
				err = handle.Write(ctx, buf)
				if err != nil {
					r.logger.Fatalf("i2c handle write failed %s", err)
					return
				}
				// close i2c handle
				err = handle.Close()
				if err != nil {
					r.logger.Fatalf("failed to close handle: %s", err)
					return
				}
			}
		}
	})
}

// Close shuts down the rtkStation.
func (r *rtkStation) Close() error {
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
		err := port.(io.ReadWriteCloser).Close()
		if err != nil {
			return err
		}
	}

	r.logger.Debug("RTK Station Closed")
	return nil
}

// ReadLocation implements a LocalGPS function, but returns nil since the rtkStation does not have GPS data.
func (r *rtkStation) ReadLocation(ctx context.Context) (*geo.Point, error) {
	r.logger.Info("Reading location of station")
	return &geo.Point{}, nil
}

// ReadAltitude implements a LocalGPS function, but returns 0 since the rtkStation does not have GPS data.
func (r *rtkStation) ReadAltitude(ctx context.Context) (float64, error) {
	r.logger.Info("Reading altitude of station")
	return 0, nil
}

// ReadSpeed implements a LocalGPS function, but returns 0 since the rtkStation does not have GPS data.
func (r *rtkStation) ReadSpeed(ctx context.Context) (float64, error) {
	r.logger.Info("Reading speed of station")
	return 0, nil
}

// ReadSatellites implements a LocalGPS function, but returns 0, 0 since the rtkStation does not have GPS data.
func (r *rtkStation) ReadSatellites(ctx context.Context) (int, int, error) {
	r.logger.Info("Reading number of satellites visible of station")
	return 0, 0, nil
}

// ReadAccuracy implements a LocalGPS function, but returns 0, 0 since the rtkStation does not have GPS data.
func (r *rtkStation) ReadAccuracy(ctx context.Context) (float64, float64, error) {
	r.logger.Info("Reading accuracy of station")
	return 0, 0, nil
}

// ReadValid implements a LocalGPS function, but returns false since the rtkStation does not have GPS data.
func (r *rtkStation) ReadValid(ctx context.Context) (bool, error) {
	return false, nil
}
