// Package ams implements the AMS_AS5048 encoder
package ams

import (
	"context"
	"math"
	"sync"
	"time"

	"github.com/edaniels/golog"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	"go.viam.com/utils"

	"go.viam.com/rdk/components/board"
	"go.viam.com/rdk/components/encoder"
	"go.viam.com/rdk/components/generic"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/resource"
	rdkutils "go.viam.com/rdk/utils"
)

const (
	i2cConn           = "i2c"
	transitionEpsilon = 90
)

var (
	modelName            = resource.NewDefaultModel("AMS-AS5048")
	scalingFactor        = 360.0 / math.Pow(2, 14)
	supportedConnections = utils.NewStringSet(i2cConn)
)

// the wait time necessary to operate the position updating
// loop at 50 Hz.
var waitTimeNano = (1.0 / 50.0) * 1000000000.0

func init() {
	registry.RegisterComponent(
		encoder.Subtype,
		modelName,
		registry.Component{
			Constructor: func(
				ctx context.Context,
				deps registry.Dependencies,
				config config.Component,
				logger golog.Logger,
			) (interface{}, error) {
				return newAS5048Encoder(ctx, deps, config, logger)
			},
		},
	)
	config.RegisterComponentAttributeMapConverter(
		encoder.Subtype,
		modelName,
		func(attributes config.AttributeMap) (interface{}, error) {
			var conf AttrConfig
			return config.TransformAttributeMapToStruct(&conf, attributes)
		},
		&AttrConfig{},
	)
}

// AttrConfig contains the connection information for
// configuring an AS5048 encoder.
type AttrConfig struct {
	BoardName string `json:"board"`
	// We include connection type here in anticipation for
	// future SPI support
	ConnectionType string `json:"connection_type"`
	*I2CAttrConfig `json:"i2c_attributes,omitempty"`
}

// Validate checks the attributes of an initialized config
// for proper values.
func (conf *AttrConfig) Validate(path string) ([]string, error) {
	var deps []string

	connType := conf.ConnectionType
	if len(connType) == 0 {
		// TODO: stop defaulting to I2C when SPI support is implemented
		conf.ConnectionType = i2cConn
		// return nil, errors.New("must specify connection type")
	}
	_, isSupported := supportedConnections[connType]
	if !isSupported {
		return nil, errors.Errorf("%s is not a supported connection type", connType)
	}
	if connType == i2cConn {
		if len(conf.BoardName) == 0 {
			return nil, errors.New("expected nonempty board")
		}
		if conf.I2CAttrConfig == nil {
			return nil, errors.New("i2c selected as connection type, but no attributes supplied")
		}
		err := conf.I2CAttrConfig.ValidateI2C(path)
		if err != nil {
			return nil, err
		}
		deps = append(deps, conf.BoardName)
	}

	return deps, nil
}

// I2CAttrConfig stores the configuration information for I2C connection.
type I2CAttrConfig struct {
	I2CBus  string `json:"i2c_bus"`
	I2CAddr int    `json:"i2c_addr"`
}

// ValidateI2C ensures all parts of the config are valid.
func (cfg *I2CAttrConfig) ValidateI2C(path string) error {
	if cfg.I2CBus == "" {
		return utils.NewConfigValidationFieldRequiredError(path, "i2c_bus")
	}
	if cfg.I2CAddr == 0 {
		return utils.NewConfigValidationFieldRequiredError(path, "i2c_addr")
	}

	return nil
}

// Encoder is a struct representing an instance of a hardware unit
// in AMS's AS5048 series of Hall-effect encoders.
type Encoder struct {
	mu                      sync.RWMutex
	logger                  golog.Logger
	position                float64
	positionOffset          float64
	rotations               int
	connectionType          string
	positionType            encoder.PositionType
	i2cBus                  board.I2C
	i2cAddr                 byte
	cancelCtx               context.Context
	cancel                  context.CancelFunc
	activeBackgroundWorkers sync.WaitGroup
	generic.Unimplemented
}

func newAS5048Encoder(
	ctx context.Context, deps registry.Dependencies,
	cfg config.Component, logger *zap.SugaredLogger,
) (encoder.Encoder, error) {
	attr, ok := cfg.ConvertedAttributes.(*AttrConfig)
	if !ok {
		return nil, rdkutils.NewUnexpectedTypeError(attr, cfg.ConvertedAttributes)
	}
	cancelCtx, cancel := context.WithCancel(ctx)
	res := &Encoder{
		connectionType: attr.ConnectionType,
		cancelCtx:      cancelCtx,
		cancel:         cancel,
		logger:         logger,
		positionType:   encoder.PositionTypeTICKS,
	}
	brd, err := board.FromDependencies(deps, attr.BoardName)
	if err != nil {
		return nil, err
	}
	localBoard, ok := brd.(board.LocalBoard)
	if !ok {
		return nil, errors.Errorf(
			"board with name %s does not implement the LocalBoard interface", attr.BoardName,
		)
	}
	if res.connectionType == i2cConn {
		i2c, exists := localBoard.I2CByName(attr.I2CBus)
		if !exists {
			return nil, errors.Errorf("unable to find I2C bus: %s", attr.I2CBus)
		}
		res.i2cBus = i2c
		res.i2cAddr = byte(attr.I2CAddr)
	}
	if err := res.startPositionLoop(ctx); err != nil {
		return nil, err
	}
	return res, nil
}

func (enc *Encoder) startPositionLoop(ctx context.Context) error {
	if err := enc.ResetPosition(ctx, map[string]interface{}{}); err != nil {
		return err
	}
	enc.activeBackgroundWorkers.Add(1)
	utils.ManagedGo(func() {
		for {
			if enc.cancelCtx.Err() != nil {
				return
			}
			if err := enc.updatePosition(ctx); err != nil {
				enc.logger.Errorf(
					"error in position loop (skipping update): %s", err.Error(),
				)
			}
			time.Sleep(time.Duration(waitTimeNano))
		}
	}, enc.activeBackgroundWorkers.Done)
	return nil
}

func (enc *Encoder) readPosition(ctx context.Context) (float64, error) {
	// retrieve the 8 most significant bits of the 14-bit resolution
	// position
	msB, err := enc.readByteDataFromBus(ctx, enc.i2cBus, enc.i2cAddr, byte(0xFE))
	if err != nil {
		return 0, err
	}
	// retrieve the 6 least significant bits of as a byte (where
	// the front two bits are irrelevant)
	lsB, err := enc.readByteDataFromBus(ctx, enc.i2cBus, enc.i2cAddr, byte(0xFF))
	if err != nil {
		return 0, err
	}
	return convertBytesToAngle(msB, lsB), nil
}

func convertBytesToAngle(msB, lsB byte) float64 {
	// obtain the 14-bit resolution position, which represents a
	// portion of a full rotation. We then scale appropriately
	// by (360 / 2^14) to get degrees
	byteData := (int(msB) << 6) | int(lsB)
	return (float64(byteData) * scalingFactor)
}

func (enc *Encoder) updatePosition(ctx context.Context) error {
	enc.mu.Lock()
	defer enc.mu.Unlock()
	angleDeg, err := enc.readPosition(ctx)
	if err != nil {
		return err
	}
	angleDeg += enc.positionOffset
	// in order to keep track of multiple rotations, we increment / decrement
	// a rotations counter whenever two subsequent positions are on either side
	// of 0 (or 360) within a window of 2 * transitionEpsilon
	forwardsTransition := (angleDeg <= transitionEpsilon) && ((360.0 - enc.position) <= transitionEpsilon)
	backwardsTransition := (enc.position <= transitionEpsilon) && ((360.0 - angleDeg) <= transitionEpsilon)
	if forwardsTransition {
		enc.rotations++
	} else if backwardsTransition {
		enc.rotations--
	}
	enc.position = angleDeg
	return nil
}

// GetPosition returns the total number of rotations detected
// by the encoder (rather than a number of pulse state transitions)
// because this encoder is absolute and not incremental. As a result
// a user MUST set ticks_per_rotation on the config of the corresponding
// motor to 1. Any other value will result in completely incorrect
// position measurements by the motor.
func (enc *Encoder) GetPosition(
	ctx context.Context, positionType *encoder.PositionType, extra map[string]interface{},
) (float64, encoder.PositionType, error) {
	enc.mu.RLock()
	defer enc.mu.RUnlock()
	if positionType != nil && *positionType == encoder.PositionTypeDEGREES {
		enc.positionType = encoder.PositionTypeDEGREES
		return enc.position, enc.positionType, nil
	}
	ticks := float64(enc.rotations) + enc.position/360.0
	enc.positionType = encoder.PositionTypeTICKS
	return ticks, enc.positionType, nil
}

// ResetPosition sets the current position measured by the encoder to be
// considered its new zero position.
func (enc *Encoder) ResetPosition(
	ctx context.Context, extra map[string]interface{},
) error {
	enc.mu.Lock()
	defer enc.mu.Unlock()
	// NOTE (GV): potential improvement could be writing the offset position
	// to the zero register of the encoder rather than keeping track
	// on the struct
	enc.position = 0.0
	currentMSB, err := enc.readByteDataFromBus(ctx, enc.i2cBus, enc.i2cAddr, byte(0xFE))
	if err != nil {
		return err
	}
	currentLSB, err := enc.readByteDataFromBus(ctx, enc.i2cBus, enc.i2cAddr, byte(0xFF))
	if err != nil {
		return err
	}
	// clear current zero position
	err = enc.writeByteDataToBus(ctx, enc.i2cBus, enc.i2cAddr, byte(0x16), byte(0))
	if err != nil {
		return err
	}
	err = enc.writeByteDataToBus(ctx, enc.i2cBus, enc.i2cAddr, byte(0x17), byte(0))
	if err != nil {
		return err
	}
	// write current position to zero register
	err = enc.writeByteDataToBus(ctx, enc.i2cBus, enc.i2cAddr, byte(0x16), currentMSB)
	if err != nil {
		return err
	}
	err = enc.writeByteDataToBus(ctx, enc.i2cBus, enc.i2cAddr, byte(0x17), currentLSB)
	if err != nil {
		return err
	}
	return nil
}

// GetProperties returns a list of all the position types that are supported by a given encoder.
func (enc *Encoder) GetProperties(ctx context.Context, extra map[string]interface{}) (map[encoder.Feature]bool, error) {
	return map[encoder.Feature]bool{
		encoder.TicksCountSupported:   true,
		encoder.AngleDegreesSupported: true,
	}, nil
}

// Close stops the position loop of the encoder when the component
// is closed.
func (enc *Encoder) Close() error {
	enc.cancel()
	enc.activeBackgroundWorkers.Wait()
	return nil
}

// readByteDataFromBus opens a handle for the bus adhoc to perform a single read
// and returns the result. The handle is closed at the end.
func (enc *Encoder) readByteDataFromBus(ctx context.Context, bus board.I2C, addr, register byte) (byte, error) {
	i2cHandle, err := bus.OpenHandle(addr)
	if err != nil {
		return 0, err
	}
	defer func() {
		if err := i2cHandle.Close(); err != nil {
			enc.logger.Error(err)
		}
	}()
	return i2cHandle.ReadByteData(ctx, register)
}

// writeByteDataToBus opens a handle for the bus adhoc to perform a single write.
// The handle is closed at the end.
func (enc *Encoder) writeByteDataToBus(ctx context.Context, bus board.I2C, addr, register, data byte) error {
	i2cHandle, err := bus.OpenHandle(addr)
	if err != nil {
		return err
	}
	defer func() {
		if err := i2cHandle.Close(); err != nil {
			enc.logger.Error(err)
		}
	}()
	return i2cHandle.WriteByteData(ctx, register, data)
}
