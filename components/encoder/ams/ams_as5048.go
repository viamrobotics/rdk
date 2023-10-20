// Package ams implements the AMS_AS5048 encoder
package ams

import (
	"context"
	"math"
	"sync"
	"time"

	"github.com/pkg/errors"
	"go.viam.com/utils"

	"go.viam.com/rdk/components/board"
	"go.viam.com/rdk/components/encoder"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
)

const (
	i2cConn           = "i2c"
	transitionEpsilon = 90
)

var (
	model                = resource.DefaultModelFamily.WithModel("AMS-AS5048")
	scalingFactor        = 360.0 / math.Pow(2, 14)
	supportedConnections = utils.NewStringSet(i2cConn)
)

// the wait time necessary to operate the position updating
// loop at 50 Hz.
var waitTimeNano = (1.0 / 50.0) * 1000000000.0

func init() {
	resource.RegisterComponent(
		encoder.API,
		model,
		resource.Registration[encoder.Encoder, *Config]{
			Constructor: newAS5048Encoder,
		},
	)
}

// Config contains the connection information for
// configuring an AS5048 encoder.
type Config struct {
	BoardName string `json:"board"`
	// We include connection type here in anticipation for
	// future SPI support
	ConnectionType string `json:"connection_type"`
	*I2CConfig     `json:"i2c_attributes,omitempty"`
}

// Validate checks the attributes of an initialized config
// for proper values.
func (conf *Config) Validate(path string) ([]string, error) {
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
		if conf.I2CConfig == nil {
			return nil, errors.New("i2c selected as connection type, but no attributes supplied")
		}
		err := conf.I2CConfig.ValidateI2C(path)
		if err != nil {
			return nil, err
		}
		deps = append(deps, conf.BoardName)
	}

	return deps, nil
}

// I2CConfig stores the configuration information for I2C connection.
type I2CConfig struct {
	I2CBus  string `json:"i2c_bus"`
	I2CAddr int    `json:"i2c_addr"`
}

// ValidateI2C ensures all parts of the config are valid.
func (cfg *I2CConfig) ValidateI2C(path string) error {
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
	resource.Named
	mu                      sync.RWMutex
	logger                  logging.Logger
	position                float64
	positionOffset          float64
	rotations               int
	positionType            encoder.PositionType
	i2cBus                  board.I2C
	i2cAddr                 byte
	cancelCtx               context.Context
	cancel                  context.CancelFunc
	activeBackgroundWorkers sync.WaitGroup
}

func newAS5048Encoder(
	ctx context.Context,
	deps resource.Dependencies,
	conf resource.Config,
	logger logging.ZapCompatibleLogger,
) (encoder.Encoder, error) {
	cancelCtx, cancel := context.WithCancel(context.Background())
	res := &Encoder{
		Named:        conf.ResourceName().AsNamed(),
		cancelCtx:    cancelCtx,
		cancel:       cancel,
		logger:       logging.FromZapCompatible(logger),
		positionType: encoder.PositionTypeTicks,
	}
	if err := res.Reconfigure(ctx, deps, conf); err != nil {
		return nil, err
	}
	if err := res.startPositionLoop(ctx); err != nil {
		return nil, err
	}
	return res, nil
}

// Reconfigure reconfigures the encoder atomically.
func (enc *Encoder) Reconfigure(
	ctx context.Context,
	deps resource.Dependencies,
	conf resource.Config,
) error {
	newConf, err := resource.NativeConfig[*Config](conf)
	if err != nil {
		return err
	}

	brd, err := board.FromDependencies(deps, newConf.BoardName)
	if err != nil {
		return err
	}
	localBoard, ok := brd.(board.LocalBoard)
	if !ok {
		return errors.Errorf(
			"board with name %s does not implement the LocalBoard interface", newConf.BoardName,
		)
	}
	i2c, exists := localBoard.I2CByName(newConf.I2CBus)
	if !exists {
		return errors.Errorf("unable to find I2C bus: %s", newConf.I2CBus)
	}
	enc.mu.Lock()
	defer enc.mu.Unlock()
	if enc.i2cBus == i2c || enc.i2cAddr == byte(newConf.I2CAddr) {
		return nil
	}
	enc.i2cBus = i2c
	enc.i2cAddr = byte(newConf.I2CAddr)
	return nil
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
			if err := enc.updatePosition(enc.cancelCtx); err != nil {
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
	i2cHandle, err := enc.i2cBus.OpenHandle(enc.i2cAddr)
	if err != nil {
		return 0, err
	}
	defer utils.UncheckedErrorFunc(i2cHandle.Close)

	// retrieve the 8 most significant bits of the 14-bit resolution
	// position
	msB, err := i2cHandle.ReadByteData(ctx, byte(0xFE))
	if err != nil {
		return 0, err
	}
	// retrieve the 6 least significant bits of as a byte (where
	// the front two bits are irrelevant)
	lsB, err := i2cHandle.ReadByteData(ctx, byte(0xFF))
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

// Position returns the total number of rotations detected
// by the encoder (rather than a number of pulse state transitions)
// because this encoder is absolute and not incremental. As a result
// a user MUST set ticks_per_rotation on the config of the corresponding
// motor to 1. Any other value will result in completely incorrect
// position measurements by the motor.
func (enc *Encoder) Position(
	ctx context.Context, positionType encoder.PositionType, extra map[string]interface{},
) (float64, encoder.PositionType, error) {
	enc.mu.RLock()
	defer enc.mu.RUnlock()
	if positionType == encoder.PositionTypeDegrees {
		enc.positionType = encoder.PositionTypeDegrees
		return enc.position, enc.positionType, nil
	}
	ticks := float64(enc.rotations) + enc.position/360.0
	enc.positionType = encoder.PositionTypeTicks
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
	enc.position = 0
	enc.rotations = 0

	i2cHandle, err := enc.i2cBus.OpenHandle(enc.i2cAddr)
	if err != nil {
		return err
	}
	defer utils.UncheckedErrorFunc(i2cHandle.Close)

	// clear current zero position
	if err := i2cHandle.WriteByteData(ctx, byte(0x16), byte(0)); err != nil {
		return err
	}
	if err := i2cHandle.WriteByteData(ctx, byte(0x17), byte(0)); err != nil {
		return err
	}

	// read current position
	currentMSB, err := i2cHandle.ReadByteData(ctx, byte(0xFE))
	if err != nil {
		return err
	}
	currentLSB, err := i2cHandle.ReadByteData(ctx, byte(0xFF))
	if err != nil {
		return err
	}

	// write current position to zero register
	if err := i2cHandle.WriteByteData(ctx, byte(0x16), currentMSB); err != nil {
		return err
	}
	if err := i2cHandle.WriteByteData(ctx, byte(0x17), currentLSB); err != nil {
		return err
	}

	return nil
}

// Properties returns a list of all the position types that are supported by a given encoder.
func (enc *Encoder) Properties(ctx context.Context, extra map[string]interface{}) (encoder.Properties, error) {
	return encoder.Properties{
		TicksCountSupported:   true,
		AngleDegreesSupported: true,
	}, nil
}

// Close stops the position loop of the encoder when the component
// is closed.
func (enc *Encoder) Close(ctx context.Context) error {
	enc.cancel()
	enc.activeBackgroundWorkers.Wait()
	return nil
}
