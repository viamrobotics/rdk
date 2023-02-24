// Package gpsrtk defines a gps and an rtk correction source
// which sends rtcm data to a child gps
// This is an Experimental package
package gpsrtk

import (
	"context"
	"fmt"
	"io"
	"sync"

	"github.com/edaniels/golog"
	"github.com/golang/geo/r3"
	"github.com/jacobsa/go-serial/serial"
	geo "github.com/kellydunn/golang-geo"
	"github.com/pkg/errors"
	"go.viam.com/utils"

	"go.viam.com/rdk/components/board"
	"go.viam.com/rdk/components/generic"
	"go.viam.com/rdk/components/movementsensor"
	"go.viam.com/rdk/components/movementsensor/gpsnmea"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/spatialmath"
	rdkutils "go.viam.com/rdk/utils"
)

// StationConfig is used for converting RTK MovementSensor config attributes.
type StationConfig struct {
	CorrectionSource string   `json:"correction_source"`
	Children         []string `json:"children,omitempty"`
	Board            string   `json:"board,omitempty"`

	// non ntrip
	SurveyIn         string  `json:"svin,omitempty"`
	RequiredAccuracy float64 `json:"required_accuracy,omitempty"` // fixed number 1-5, 5 being the highest accuracy
	RequiredTime     int     `json:"required_time_sec,omitempty"`

	*SerialAttrConfig `json:"serial_attributes,omitempty"`
	*I2CAttrConfig    `json:"i2c_attributes,omitempty"`
	*NtripAttrConfig  `json:"ntrip_attributes,omitempty"`
}

const (
	i2cStr    = "i2c"
	serialStr = "serial"
	ntripStr  = "ntrip"
	timeMode  = "time"
)

// ErrStationValidation contains the model substring for the available correction source types.
var ErrStationValidation = fmt.Errorf("only serial, I2C, and ntrip are supported correction sources for %s", stationModel.Name)

// Validate ensures all parts of the config are valid.
func (cfg *StationConfig) Validate(path string) ([]string, error) {
	var deps []string

	// not ntrip, using serial or i2c for correction source
	if cfg.SurveyIn == timeMode {
		if cfg.RequiredAccuracy == 0 {
			return nil, utils.NewConfigValidationFieldRequiredError(path, "required_accuracy")
		}
		if cfg.RequiredTime == 0 {
			return nil, utils.NewConfigValidationFieldRequiredError(path, "required_time")
		}
	}

	switch cfg.CorrectionSource {
	case ntripStr:
		return nil, cfg.NtripAttrConfig.ValidateNtrip(path)
	case i2cStr:
		if cfg.Board == "" {
			return nil, utils.NewConfigValidationFieldRequiredError(path, "board")
		}
		deps = append(deps, cfg.Board)
		return deps, cfg.I2CAttrConfig.ValidateI2C(path)
	case serialStr:
		if cfg.SerialAttrConfig.SerialCorrectionPath == "" {
			return nil, utils.NewConfigValidationFieldRequiredError(path, "serial_correction_path")
		}
	case "":
		return nil, utils.NewConfigValidationFieldRequiredError(path, "correction_source")
	default:
		return nil, ErrStationValidation
	}
	return deps, nil
}

var stationModel = resource.NewDefaultModel("rtk-station")

func init() {
	registry.RegisterComponent(
		movementsensor.Subtype,
		stationModel,
		registry.Component{Constructor: func(
			ctx context.Context,
			deps registry.Dependencies,
			cfg config.Component,
			logger golog.Logger,
		) (interface{}, error) {
			return newRTKStation(ctx, deps, cfg, logger)
		}})

	config.RegisterComponentAttributeMapConverter(movementsensor.Subtype, stationModel,
		func(attributes config.AttributeMap) (interface{}, error) {
			var attr StationConfig
			return config.TransformAttributeMapToStruct(&attr, attributes)
		},
		&StationConfig{})
}

type rtkStation struct {
	generic.Unimplemented
	logger              golog.Logger
	correction          correctionSource
	correctionType      string
	i2cPaths            []i2cBusAddr
	serialPorts         []io.Writer
	serialWriter        io.Writer
	movementsensorNames []string

	cancelCtx               context.Context
	cancelFunc              func()
	activeBackgroundWorkers sync.WaitGroup

	err movementsensor.LastError
}

type correctionSource interface {
	Reader() (io.ReadCloser, error)
	Start(ready chan<- bool)
	Close() error
}

type i2cBusAddr struct {
	bus  board.I2C
	addr byte
}

func newRTKStation(
	ctx context.Context,
	deps registry.Dependencies,
	cfg config.Component,
	logger golog.Logger,
) (movementsensor.MovementSensor, error) {
	attr, ok := cfg.ConvertedAttributes.(*StationConfig)
	if !ok {
		return nil, rdkutils.NewUnexpectedTypeError(attr, cfg.ConvertedAttributes)
	}

	cancelCtx, cancelFunc := context.WithCancel(ctx)

	r := &rtkStation{
		cancelCtx:  cancelCtx,
		cancelFunc: cancelFunc,
		logger:     logger,
	}

	r.correctionType = attr.CorrectionSource

	// Init correction source
	var err error
	switch r.correctionType {
	case ntripStr:
		r.correction, err = newNtripCorrectionSource(ctx, cfg, logger)
		if err != nil {
			return nil, err
		}
	case serialStr:
		r.correction, err = newSerialCorrectionSource(ctx, cfg, logger)
		if err != nil {
			return nil, err
		}
	case i2cStr:
		r.correction, err = newI2CCorrectionSource(ctx, deps, cfg, logger)
		if err != nil {
			return nil, err
		}
	default:
		// Invalid source
		return nil, fmt.Errorf("%s is not a valid correction source", r.correctionType)
	}

	r.movementsensorNames = attr.Children

	err = ConfigureBaseRTKStation(cfg)
	if err != nil {
		r.logger.Info("rtk base station could not be configured")
		return r, err
	}

	// Init movementsensor correction input addresses
	r.logger.Debug("Init movementsensor")
	r.serialPorts = make([]io.Writer, 0)

	for _, movementsensorName := range r.movementsensorNames {
		movementsensor, err := movementsensor.FromDependencies(deps, movementsensorName)
		localmovementsensor := rdkutils.UnwrapProxy(movementsensor)
		if err != nil {
			return nil, err
		}

		switch t := localmovementsensor.(type) {
		case *gpsnmea.SerialNMEAMovementSensor:
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

		case *gpsnmea.PmtkI2CNMEAMovementSensor:
			bus, addr := t.GetBusAddr()
			busAddr := i2cBusAddr{bus: bus, addr: addr}

			r.i2cPaths = append(r.i2cPaths, busAddr)
		default:
			return nil, errors.New("child is not valid gpsnmeaMovementSensor type")
		}
	}

	r.logger.Debug("Init multiwriter")
	r.serialWriter = io.MultiWriter(r.serialPorts...)
	r.logger.Debug("Starting")

	r.Start(ctx)
	return r, r.err.Get()
}

// Start starts reading from the correction source and sends corrections to the child movementsensor's.
func (r *rtkStation) Start(ctx context.Context) {
	r.activeBackgroundWorkers.Add(1)
	utils.PanicCapturingGo(func() {
		defer r.activeBackgroundWorkers.Done()

		// read from correction source
		ready := make(chan bool)
		go r.correction.Start(ready)

		<-ready
		stream, err := r.correction.Reader()
		if err != nil {
			r.logger.Errorf("Unable to get reader: %s", err)
			r.err.Set(err)
			return
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
				r.logger.Errorf("Unable to read stream: %s", err)
				r.err.Set(err)
				return
			}

			// write buf to all i2c handles
			for _, busAddr := range r.i2cPaths {
				// open handle
				handle, err := busAddr.bus.OpenHandle(busAddr.addr)
				if err != nil {
					r.logger.Errorf("can't open movementsensor i2c handle: %s", err)
					r.err.Set(err)
					return
				}
				// write to i2c handle
				err = handle.Write(ctx, buf)
				if err != nil {
					r.logger.Errorf("i2c handle write failed %s", err)
					r.err.Set(err)
					return
				}
				// close i2c handle
				err = handle.Close()
				if err != nil {
					r.logger.Errorf("failed to close handle: %s", err)
					r.err.Set(err)
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
	return r.err.Get()
}

func (r *rtkStation) Position(ctx context.Context, extra map[string]interface{}) (*geo.Point, float64, error) {
	return &geo.Point{}, 0, movementsensor.ErrMethodUnimplementedPosition
}

func (r *rtkStation) LinearVelocity(ctx context.Context, extra map[string]interface{}) (r3.Vector, error) {
	return r3.Vector{}, movementsensor.ErrMethodUnimplementedLinearVelocity
}

func (r *rtkStation) LinearAcceleration(ctx context.Context, extra map[string]interface{}) (r3.Vector, error) {
	return r3.Vector{}, movementsensor.ErrMethodUnimplementedLinearAcceleration
}

func (r *rtkStation) AngularVelocity(ctx context.Context, extra map[string]interface{}) (spatialmath.AngularVelocity, error) {
	return spatialmath.AngularVelocity{}, movementsensor.ErrMethodUnimplementedAngularVelocity
}

func (r *rtkStation) Orientation(ctx context.Context, extra map[string]interface{}) (spatialmath.Orientation, error) {
	return spatialmath.NewZeroOrientation(), movementsensor.ErrMethodUnimplementedOrientation
}

func (r *rtkStation) CompassHeading(ctx context.Context, extra map[string]interface{}) (float64, error) {
	return 0, movementsensor.ErrMethodUnimplementedCompassHeading
}

func (r *rtkStation) Readings(ctx context.Context, extra map[string]interface{}) (map[string]interface{}, error) {
	return map[string]interface{}{}, movementsensor.ErrMethodUnimplementedReadings
}

func (r *rtkStation) Accuracy(ctx context.Context, extra map[string]interface{}) (map[string]float32, error) {
	return map[string]float32{}, movementsensor.ErrMethodUnimplementedAccuracy
}

func (r *rtkStation) Properties(ctx context.Context, extra map[string]interface{}) (*movementsensor.Properties, error) {
	return &movementsensor.Properties{}, r.err.Get()
}
