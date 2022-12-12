package encoder

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
	"go.viam.com/rdk/components/generic"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/resource"
	rdkutils "go.viam.com/rdk/utils"
)

var am5ModelName = resource.NewDefaultModel("AS5048A")

const i2cConn = "I2C"

var supportedConnections = utils.NewStringSet(i2cConn)

const transitionEpsilon = 90

var scalingFactor = 360.0 / math.Pow(2, 14)

func init() {
	registry.RegisterComponent(
		Subtype,
		am5ModelName,
		registry.Component{
			Constructor: func(
				ctx context.Context,
				deps registry.Dependencies,
				config config.Component,
				logger golog.Logger,
			) (interface{}, error) {
				return NewAS5048AEncoder(ctx, deps, config, logger)
			},
		},
	)
	config.RegisterComponentAttributeMapConverter(
		Subtype,
		am5ModelName,
		func(attributes config.AttributeMap) (interface{}, error) {
			var conf AS5048AConfig
			return config.TransformAttributeMapToStruct(&conf, attributes)
		},
		&AS5048AConfig{},
	)
}

// AS5048AConfig contains the connection information for
// configuring the AS5048A encoder.
type AS5048AConfig struct {
	BoardName string `json:"board"`
	// We include connection type here in anticipation for
	// future SPI support
	ConnectionType string `json:"connection_type"`
	*i2cAttrConfig `json:"i2c_attributes,omitempty"`
}

// Validate checks the attributes of an initialized config
// for proper values.
func (conf *AS5048AConfig) Validate(path string) ([]string, error) {
	var deps []string

	connType := conf.ConnectionType
	if len(connType) == 0 {
		// default to I2C until SPI support is implemented
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
		err := conf.i2cAttrConfig.ValidateI2C(path)
		if err != nil {
			return nil, err
		}
		deps = append(deps, conf.BoardName)
	}

	return deps, nil
}

type i2cAttrConfig struct {
	I2CBus  string `json:"i2c_bus"`
	I2CAddr int    `json:"i2c_addr"`
}

// ValidateI2C ensures all parts of the config are valid.
func (cfg *i2cAttrConfig) ValidateI2C(path string) error {
	if cfg.I2CBus == "" {
		return utils.NewConfigValidationFieldRequiredError(path, "i2c_bus")
	}
	if cfg.I2CAddr == 0 {
		return utils.NewConfigValidationFieldRequiredError(path, "i2c_addr")
	}

	return nil
}

// AS5048A is a struct representing an instance of an AS5048A
// Hall-effect encoder.
type AS5048A struct {
	mu                      sync.RWMutex
	logger                  golog.Logger
	position                float64
	positionOffset          float64
	rotations               int
	connectionType          string
	i2cHandle               board.I2CHandle
	cancelCtx               context.Context
	cancel                  context.CancelFunc
	activeBackgroundWorkers sync.WaitGroup
	generic.Unimplemented
}

func (enc *AS5048A) startPositionLoop(ctx context.Context) error {
	if err := enc.Reset(ctx, 0.0, map[string]interface{}{}); err != nil {
		return err
	}
	enc.activeBackgroundWorkers.Add(1)
	utils.ManagedGo(func() {
		for {
			if enc.cancelCtx.Err() != nil {
				return
			}
			if err := enc.updatePosition(ctx); err != nil {
				enc.logger.Errorf("error in position loop: %s", err.Error())
			}
			// operate loop at 50 Hz
			waitTimeNano := (1.0 / 50.0) * 1000000000.0
			time.Sleep(time.Duration(waitTimeNano))
		}
	}, enc.activeBackgroundWorkers.Done)
	return nil
}

func (enc *AS5048A) readPosition(ctx context.Context) (float64, error) {
	// retrieve the 8 most significant bits of the 14-bit resolution
	// position
	msB, err := enc.i2cHandle.ReadByteData(ctx, byte(0xFE))
	if err != nil {
		return 0, err
	}
	// retrieve the 6 least significant bits of as a byte (where
	// the front two bits are irrelevant)
	lsB, err := enc.i2cHandle.ReadByteData(ctx, byte(0xFF))
	if err != nil {
		return 0, err
	}
	// obtain the 14-bit resolution position, which represents a
	// portion of a full rotation. We then scale appropriately
	// by (360 / 2^14) to get degrees
	byteData := (int(msB) << 6) | int(lsB)
	return (float64(byteData) * scalingFactor), nil
}

func (enc *AS5048A) updatePosition(ctx context.Context) error {
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

// TicksCount returns the total number of rotations detected
// by the encoder (rather than a number of pulse state transitions)
// because this encoder is absolute and not incremental. As a result
// a user MUST set ticks_per_rotation on the config of the corresponding
// motor to 1. Any other value will result in completely incorrect
// position measurements by the motor.
func (enc *AS5048A) TicksCount(
	ctx context.Context, extra map[string]interface{},
) (float64, error) {
	enc.mu.RLock()
	defer enc.mu.RUnlock()
	ticks := float64(enc.rotations) + enc.position/360.0
	return ticks, nil
}

// Reset sets the current position measured by the encoder to be considered
// its new zero position. If the offset provided is not 0.0, it also
// sets the positionOffset attribute and adjusts all future recorded
// positions by that offset (until the function is called again).
func (enc *AS5048A) Reset(
	ctx context.Context, offset float64, extra map[string]interface{},
) error {
	enc.mu.Lock()
	defer enc.mu.Unlock()
	enc.positionOffset = offset
	enc.position = 0.0 + offset
	currentMSB, err := enc.i2cHandle.ReadByteData(ctx, byte(0xFE))
	if err != nil {
		return err
	}
	currentLSB, err := enc.i2cHandle.ReadByteData(ctx, byte(0xFF))
	if err != nil {
		return err
	}
	// clear current zero position
	err = enc.i2cHandle.WriteByteData(ctx, byte(0x16), byte(0))
	if err != nil {
		return err
	}
	err = enc.i2cHandle.WriteByteData(ctx, byte(0x17), byte(0))
	if err != nil {
		return err
	}
	// write current position to zero register
	err = enc.i2cHandle.WriteByteData(ctx, byte(0x16), currentMSB)
	if err != nil {
		return err
	}
	err = enc.i2cHandle.WriteByteData(ctx, byte(0x17), currentLSB)
	if err != nil {
		return err
	}
	return nil
}

// Close stops the position loop of the encoder when the component
// is closed.
func (enc *AS5048A) Close() {
	enc.cancel()
	enc.activeBackgroundWorkers.Wait()
}

// NewAS5048AEncoder returns a new instance of an AS5048A encoder.
func NewAS5048AEncoder(
	ctx context.Context, deps registry.Dependencies,
	cfg config.Component, logger *zap.SugaredLogger,
) (*AS5048A, error) {
	attr, ok := cfg.ConvertedAttributes.(*AS5048AConfig)
	if !ok {
		return nil, rdkutils.NewUnexpectedTypeError(attr, cfg.ConvertedAttributes)
	}
	cancelCtx, cancel := context.WithCancel(ctx)
	res := &AS5048A{
		connectionType: attr.ConnectionType,
		cancelCtx:      cancelCtx,
		cancel:         cancel,
		logger:         logger,
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
		i2cHandle, err := i2c.OpenHandle(byte(attr.I2CAddr))
		if err != nil {
			return nil, err
		}
		res.i2cHandle = i2cHandle
	}
	if err := res.startPositionLoop(ctx); err != nil {
		return nil, err
	}
	return res, nil
}
