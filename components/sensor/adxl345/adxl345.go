package adxl345

// The manual for this chip is available at:
// https://www.analog.com/media/en/technical-documentation/data-sheets/adxl345.pdf

import (
	"context"
	"encoding/binary"
	"sync"

	"github.com/edaniels/golog"
	"go.viam.com/utils"
	"github.com/pkg/errors"

	"go.viam.com/rdk/components/board"
	"go.viam.com/rdk/components/sensor"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/registry"
)

const modelName = "adxl345"

type AttrConfig struct {
	BoardName              string `json:"board"`
	BusId                  string `json:"bus_id"`
	UseAlternateI2CAddress bool   `json:"use_alternate_i2c_address"`
}

// Validate ensures all parts of the config are valid.
func (cfg *AttrConfig) Validate(path string) error {
	if cfg.BoardName == "" {
		return utils.NewConfigValidationFieldRequiredError(path, "board")
	}
	if cfg.BusId == "" {
		return utils.NewConfigValidationFieldRequiredError(path, "bus_id")
	}
	return nil
}

func init() {
	registry.RegisterComponent(sensor.Subtype, modelName, registry.Component{
		Constructor: func(
			ctx context.Context,
			deps registry.Dependencies,
			cfg config.Component,
			logger golog.Logger,
		) (interface{}, error) {
			return NewAdxl345(ctx, deps, cfg, logger)
		},
	})

	config.RegisterComponentAttributeMapConverter(sensor.SubtypeName, modelName,
		func(attributes config.AttributeMap) (interface{}, error) {
			var attr AttrConfig
			return config.TransformAttributeMapToStruct(&attr, attributes)
		},
		&AttrConfig{})
}

type adxl345 struct {
	bus        board.I2C
	i2cAddress byte
	mu         sync.Mutex

	logger     golog.Logger
}

func NewAdxl345(
	ctx context.Context,
	deps registry.Dependencies,
	rawConfig config.Component,
	logger golog.Logger,
) (sensor.Sensor, error) {
	cfg := rawConfig.ConvertedAttributes.(*AttrConfig)
	b, err := board.FromDependencies(deps, cfg.BoardName)
	if err != nil {
		return nil, err
	}
	localB, ok := b.(board.LocalBoard)
	if !ok {
		return nil, errors.Errorf("board %s is not local", cfg.BoardName)
	}
	bus, ok := localB.I2CByName(cfg.BusId)
	if !ok {
		return nil, errors.Errorf("can't find I2C bus '%s' for ADXL345 sensor", cfg.BusId)
	}

	var address byte
	if cfg.UseAlternateI2CAddress {
		address = 0x1D
	} else {
		address = 0x53
	}

	sensor := &adxl345{
		bus: bus,
		i2cAddress: address,
		logger: logger,
	}

	// To check that we're able to talk to the chip, we should be able to read register 0 and get
	// back the device ID (0xE5).
	deviceId, err := sensor.readByte(0, ctx)
	if err != nil {
		return nil, errors.Errorf("can't read from I2C address {} on bus {} of board {}: '{}'",
		                          address, cfg.BusId, cfg.BoardName, err.Error())
	}
	if deviceId != 0xE5 {
		return nil, errors.Errorf("unexpected I2C device instead of ADXL345: deviceID '{}'",
		                          deviceId)
	}

	// The chip starts out in standby mode. Set it to measurement mode so we can get data from it.
	// To do this, we set the Power Control register (0x2D) to turn on the 8's bit.
	err = sensor.writeByte(0x2D, 0x08, ctx)
	if (err != nil) {
		return nil, errors.Errorf("unable to put ADXL345 into measurement mode: '{}'", err.Error())
	}

	return sensor, nil
}

func (adxl *adxl345) readByte(register byte, ctx context.Context) (byte, error) {
	result, err := adxl.readBlock(register, 1, ctx)
	if err != nil {
		return 0, err
	}
	return result[0], err
}

func (adxl *adxl345) readBlock(register byte, length int, ctx context.Context) ([]byte, error) {
	handle, err := adxl.bus.OpenHandle(adxl.i2cAddress)
	if err != nil {
		return nil, err
	}
	defer func() {
		err := handle.Close()
		if err != nil {
			adxl.logger.Error(err)
		}
	}()

	results := make([]byte, length)
	results, err = handle.Read(ctx, length)
	return results, err
}

func (adxl *adxl345) writeByte(register byte, value byte, ctx context.Context) error {
	handle, err := adxl.bus.OpenHandle(adxl.i2cAddress)
	if err != nil {
		return err
	}
	defer func() {
		err := handle.Close()
		if err != nil {
			adxl.logger.Error(err)
		}
	}()

	return handle.WriteByteData(ctx, register, value)
}

// A helper function: takes 2 bytes and reinterprets them as a little-endian signed integer.
func toSignedValue(data []byte) int16 {
	return int16(binary.LittleEndian.Uint16(data))
}

func (adxl *adxl345) Readings(ctx context.Context) (map[string]interface{}, error) {
	adxl.mu.Lock()
    defer adxl.mu.Unlock()
	// The registers holding the data are 0x32 through 0x37: two bytes each for X, Y, and Z.
	rawData, err := adxl.readBlock(0x32, 6, ctx)
	if err != nil {
		return nil, err
	}

	x := toSignedValue(rawData[0:2])
	y := toSignedValue(rawData[2:4])
	z := toSignedValue(rawData[4:6])
	return map[string]interface{}{"x": x, "y": y, "z": z}, nil
}

func (adxl *adxl345) DoCommand(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	adxl.mu.Lock()
    defer adxl.mu.Unlock()
	// TODO: implement this.
	return nil, nil
}

func (adxl *adxl345) Close() {
	adxl.mu.Lock()
    defer adxl.mu.Unlock()
	// Put the chip into standby mode by setting the Power Control register (0x2D) to 0.
	err := adxl.writeByte(0x2D, 0x00, nil)
	if err != nil {
		adxl.logger.Error(err)
	}
}
