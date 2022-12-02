// Package adxl345 implements the Sensor interface for the ADXL345 accelerometer attached to the
// I2C bus of the robot (the chip supports communicating over SPI as well, but this package does
// not support that interface). The manual for this chip is available at:
// https://www.analog.com/media/en/technical-documentation/data-sheets/adxl345.pdf
package adxl345

import (
	"context"
	"encoding/binary"
	"sync"

	"github.com/edaniels/golog"
	"github.com/pkg/errors"
	"go.viam.com/utils"

	"go.viam.com/rdk/components/board"
	"go.viam.com/rdk/components/generic"
	"go.viam.com/rdk/components/sensor"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/registry"
)

const modelName = "accel-adxl345"

// AttrConfig is a description of how to find an ADXL345 accelerometer on the robot.
type AttrConfig struct {
	BoardName              string `json:"board"`
	BusID                  string `json:"bus_id"`
	UseAlternateI2CAddress bool   `json:"use_alternate_i2c_address"`
}

// Validate ensures all parts of the config are valid, and then returns the list of things we
// depend on.
func (cfg *AttrConfig) Validate(path string) ([]string, error) {
	if cfg.BoardName == "" {
		return nil, utils.NewConfigValidationFieldRequiredError(path, "board")
	}
	if cfg.BusID == "" {
		return nil, utils.NewConfigValidationFieldRequiredError(path, "bus_id")
	}
	var deps []string
	deps = append(deps, cfg.BoardName)
	return deps, nil
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
	logger     golog.Logger

	// Lock the mutex when you want to read or write either the acceleration or the last error.
	mu                 sync.Mutex
	linearAcceleration r3.Vector
	lastError          error

	// Used to shut down the background goroutine which polls the sensor.
	backgroundContext       context.Context
	cancelFunc              func()
	activeBackgroundWorkers sync.WaitGroup

	generic.Unimplemented // Implements DoCommand with an ErrUnimplemented response
}

// NewAdxl345 is a constructor to create a new object representing an ADXL345 accelerometer.
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
	bus, ok := localB.I2CByName(cfg.BusID)
	if !ok {
		return nil, errors.Errorf("can't find I2C bus '%s' for ADXL345 sensor", cfg.BusID)
	}

	var address byte
	if cfg.UseAlternateI2CAddress {
		address = 0x1D
	} else {
		address = 0x53
	}

	backgroundContext, cancelFunc := context.WithCancel(ctx)

	sensor := &adxl345{
		bus:               bus,
		i2cAddress:        address,
		logger:            logger,
		backgroundContext: backgroundContext,
		cancelFunc:        cancelFunc,
	}

	// To check that we're able to talk to the chip, we should be able to read register 0 and get
	// back the device ID (0xE5).
	deviceID, err := sensor.readByte(ctx, 0)
	if err != nil {
		return nil, errors.Errorf("can't read from I2C address %d on bus %s of board %s: '%s'",
			address, cfg.BusID, cfg.BoardName, err.Error())
	}
	if deviceID != 0xE5 {
		return nil, errors.Errorf("unexpected I2C device instead of ADXL345 at address %d: deviceID '%d'",
			address, deviceID)
	}

	// The chip starts out in standby mode. Set it to measurement mode so we can get data from it.
	// To do this, we set the Power Control register (0x2D) to turn on the 8's bit.
	err = sensor.writeByte(ctx, 0x2D, 0x08)
	if err != nil {
		return nil, errors.Errorf("unable to put ADXL345 into measurement mode: '%s'", err.Error())
	}

	// Now, turn on the background goroutine that constantly reads from the chip and stores data in
	// the object we created.
	sensor.activeBackgroundWorkers.Add(1)
	utils.PanicCapturingGo(sensor.pollData)

	return sensor, nil
}

func (adxl *adxl345) readByte(ctx context.Context, register byte) (byte, error) {
	result, err := adxl.readBlock(ctx, register, 1)
	if err != nil {
		return 0, err
	}
	return result[0], err
}

func (adxl *adxl345) readBlock(ctx context.Context, register byte, length uint8) ([]byte, error) {
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

	results, err := handle.ReadBlockData(ctx, register, length)
	return results, err
}

func (adxl *adxl345) writeByte(ctx context.Context, register, value byte) error {
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
func toSignedValue(data []byte) int {
	return int(int16(binary.LittleEndian.Uint16(data)))
}

// Given a value, scales it so that the range of int16s becomes the range of +/- maxValue.
func setScale(value int, maxValue float64) float64 {
	return float64(value) * maxValue / (1 << 15)
}

func (adxl *adxl345) pollData() {
	defer adxl.activeBackgroundWorkers.Done()
	// Reading data a thousand times per second is probably fast enough.
	timer := time.NewTicker(time.Millisecond)
	defer timer.Stop()

	for {
		select {
		case <- adxl.backgroundContext.Done():
			return
		case <- timer.C:
			// The registers holding the data are 0x32 through 0x37: two bytes each for X, Y, and Z.
			rawData, err := adxl.readBlock(ctx, 0x32, 6)
			if err != nil {
				adxl.mu.Lock()
				adxl.lastError = err
				adxl.mu.Unlock()
				continue
			}

			linearAcceleration := toLinearAcceleration(rawData)
			// Only lock the mutex to write to the shared data, so other threads can read the data
			// as often as they want.
			adxl.mu.Lock()
			adxl.linearAcceleration = linearAcceleration
			adxl.mu.Unlock()
		}
	}
}

func toLinearAcceleration(data []byte) r3.Vector {
	x := toSignedValue(data[0:2])
	y := toSignedValue(data[2:4])
	z := toSignedValue(data[4:6])

	// The default scale is +/- 2G's, but our units should be mm/sec/sec.
	maxAcceleration := 2.0 * 9.81 /* m/sec/sec */ * 1000.0 /* mm/m */
	return r3.Vector{
		X: setScale(x, maxAcceleration),
		Y: setScale(y, maxAcceleration),
		Z: setScale(z, maxAcceleration),
	}
}

func (adxl *adxl345) Readings(ctx context.Context) (map[string]interface{}, error) {
	adxl.mu.Lock()
	defer adxl.mu.Unlock()

	// We're going to return the most recent error, if any, and then clear it.
	lastError := adxl.lastError
	adxl.lastError = nil
	return map[string]interface{}{"linear_acceleration": adxl.linearAcceleration}, lastError
}

// Puts the chip into standby mode.
func (adxl *adxl345) Close(ctx context.Context) {
	adxl.mu.Lock()
	defer adxl.mu.Unlock()
	// Put the chip into standby mode by setting the Power Control register (0x2D) to 0.
	err := adxl.writeByte(ctx, 0x2D, 0x00)
	if err != nil {
		adxl.logger.Errorf("Unable to turn off ADXL345 accelerometer: '%s'", err)
	}
}
