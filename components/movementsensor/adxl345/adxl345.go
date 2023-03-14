// Package adxl345 implements the MovementSensor interface for the ADXL345 accelerometer attached
// to an I2C bus on the robot (the chip supports communicating over SPI as well, but this package
// does not support that interface). The datasheet for this chip is available at:
// https://www.analog.com/media/en/technical-documentation/data-sheets/adxl345.pdf
//
// We support reading the accelerometer data off of the chip. We do not yet support using the
// digital interrupt pins to notify on events (freefall, collision, etc.).
//
// Because we only support I2C interaction, the CS pin must be wired to hot (which tells the chip
// which communication interface to use). The chip has two possible I2C addresses, which can be
// selected by wiring the SDO pin to either hot or ground:
//   - if SDO is wired to ground, it uses the default I2C address of 0x53
//   - if SDO is wired to hot, it uses the alternate I2C address of 0x1D
//
// If you use the alternate address, your config file for this component must set its
// "use_alternate_i2c_address" boolean to true.
package adxl345

import (
	"context"
	"sync"
	"time"

	"github.com/edaniels/golog"
	"github.com/golang/geo/r3"
	geo "github.com/kellydunn/golang-geo"
	"github.com/pkg/errors"
	"go.viam.com/utils"

	"go.viam.com/rdk/components/board"
	"go.viam.com/rdk/components/generic"
	"go.viam.com/rdk/components/movementsensor"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/spatialmath"
	rutils "go.viam.com/rdk/utils"
)

var modelName = resource.NewDefaultModel("accel-adxl345")

// AttrConfig is a description of how to find an ADXL345 accelerometer on the robot.
type AttrConfig struct {
	BoardName              string              `json:"board"`
	I2cBus                 string              `json:"i2c_bus"`
	UseAlternateI2CAddress bool                `json:"use_alternate_i2c_address,omitempty"`
	SingleTap              *TapAttrConfig      `json:"tap,omitempty"`
	FreeFall               *FreeFallAttrConfig `json:"free_fall,omitempty"`
}

// TapAttrConfig is a description of the configs for tap registers.
type TapAttrConfig struct {
	AccelerometerPin int     `json:"accelerometer_pin"`
	InterruptPin     string  `json:"interrupt_pin"`
	ExcludeX         bool    `json:"exclude_x,omitempty"`
	ExcludeY         bool    `json:"exclude_y,omitempty"`
	ExcludeZ         bool    `json:"exclude_z,omitempty"`
	Threshold        float32 `json:"threshold,omitempty"`
	Dur              float32 `json:"dur_us,omitempty"`
}

// FreeFallAttrConfig is a description of the configs for free fall registers.
type FreeFallAttrConfig struct {
	AccelerometerPin int     `json:"accelerometer_pin"`
	InterruptPin     string  `json:"interrupt_pin"`
	Threshold        float32 `json:"threshold,omitempty"`
	Time             float32 `json:"time_ms,omitempty"`
}

// Validate ensures all parts of the config are valid, and then returns the list of things we
// depend on.
func (cfg *AttrConfig) Validate(path string) ([]string, error) {
	if cfg.BoardName == "" {
		return nil, utils.NewConfigValidationFieldRequiredError(path, "board")
	}
	if cfg.I2cBus == "" {
		return nil, utils.NewConfigValidationFieldRequiredError(path, "i2c_bus")
	}
	var deps []string
	deps = append(deps, cfg.BoardName)
	return deps, nil
}

func init() {
	registry.RegisterComponent(movementsensor.Subtype, modelName, registry.Component{
		Constructor: func(
			ctx context.Context,
			deps registry.Dependencies,
			cfg config.Component,
			logger golog.Logger,
		) (interface{}, error) {
			return NewAdxl345(ctx, deps, cfg, logger)
		},
	})

	config.RegisterComponentAttributeMapConverter(movementsensor.Subtype, modelName,
		func(attributes config.AttributeMap) (interface{}, error) {
			var attr AttrConfig
			return config.TransformAttributeMapToStruct(&attr, attributes)
		},
		&AttrConfig{})
}

type adxl345 struct {
	bus                      board.I2C
	i2cAddress               byte
	logger                   golog.Logger
	interruptsEnabled        byte
	interruptsFound          map[string]int
	configuredRegisterValues map[byte]byte

	// Lock the mutex when you want to read or write either the acceleration or the last error.
	mu                 sync.Mutex
	linearAcceleration r3.Vector
	err                movementsensor.LastError

	// Used to shut down the background goroutine which polls the sensor.
	cancelContext           context.Context
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
) (movementsensor.MovementSensor, error) {
	cfg, ok := rawConfig.ConvertedAttributes.(*AttrConfig)
	if !ok {
		return nil, errors.New("Cannot convert attributes to correct config type")
	}
	b, err := board.FromDependencies(deps, cfg.BoardName)
	if err != nil {
		return nil, err
	}
	localB, ok := b.(board.LocalBoard)
	if !ok {
		return nil, errors.Errorf("board %q is not local", cfg.BoardName)
	}
	bus, ok := localB.I2CByName(cfg.I2cBus)
	if !ok {
		return nil, errors.Errorf("can't find I2C bus '%q' for ADXL345 sensor", cfg.I2cBus)
	}

	var address byte
	if cfg.UseAlternateI2CAddress {
		address = 0x1D
	} else {
		address = 0x53
	}

	interruptConfigurations := getInterruptConfigurations(cfg)
	configuredRegisterValues := map[byte]byte{}
	configuredRegisterValues = getFreeFallRegisterValues(cfg.FreeFall, configuredRegisterValues)
	configuredRegisterValues = getSingleTapRegisterValues(cfg.SingleTap, configuredRegisterValues)
	cancelContext, cancelFunc := context.WithCancel(ctx)
	sensor := &adxl345{
		bus:                      bus,
		i2cAddress:               address,
		interruptsEnabled:        interruptConfigurations[IntEnableAddr],
		logger:                   logger,
		cancelContext:            cancelContext,
		cancelFunc:               cancelFunc,
		configuredRegisterValues: configuredRegisterValues,
	}

	// To check that we're able to talk to the chip, we should be able to read register 0 and get
	// back the device ID (0xE5).
	deviceID, err := sensor.readByte(ctx, 0)
	if err != nil {
		return nil, errors.Wrapf(err, "can't read from I2C address %d on bus %q of board %q",
			address, cfg.I2cBus, cfg.BoardName)
	}
	if deviceID != 0xE5 {
		return nil, errors.Errorf("unexpected I2C device instead of ADXL345 at address %d: deviceID '%d'",
			address, deviceID)
	}

	// The chip starts out in standby mode. Set it to measurement mode so we can get data from it.
	// To do this, we set the Power Control register (0x2D) to turn on the 8's bit.
	if err = sensor.writeByte(ctx, 0x2D, 0x08); err != nil {
		return nil, errors.Wrap(err, "unable to put ADXL345 into measurement mode")
	}

	// Now, turn on the background goroutine that constantly reads from the chip and stores data in
	// the object we created.
	sensor.activeBackgroundWorkers.Add(1)
	utils.PanicCapturingGo(func() {
		defer sensor.activeBackgroundWorkers.Done()
		// Reading data a thousand times per second is probably fast enough.
		timer := time.NewTicker(time.Millisecond)
		defer timer.Stop()

		for {
			select {
			case <-timer.C:
				// The registers with data are 0x32 through 0x37: two bytes each for X, Y, and Z.
				rawData, err := sensor.readBlock(sensor.cancelContext, 0x32, 6)
				if err != nil {
					sensor.mu.Lock()
					sensor.err.Set(err)
					sensor.mu.Unlock()
					continue
				}

				linearAcceleration := toLinearAcceleration(rawData)
				// Only lock the mutex to write to the shared data, so other threads can read the
				// data as often as they want.
				sensor.mu.Lock()
				sensor.linearAcceleration = linearAcceleration
				sensor.mu.Unlock()
			case <-sensor.cancelContext.Done():
				return
			}
		}
	})
	sensor.interruptsFound = make(map[string]int)
	sensor.readInterrupts(sensor.cancelContext)
	if err := sensor.configureInterruptRegisters(ctx, interruptConfigurations[IntMapAddr]); err != nil {
		sensor.cancelFunc()
		return nil, err
	}

	interruptMap := map[string]board.DigitalInterrupt{}
	if (cfg.SingleTap != nil) && (cfg.SingleTap.InterruptPin != "") {
		interruptMap, err = addInterruptPin(b, cfg.SingleTap.InterruptPin, interruptMap)
		if err != nil {
			return nil, err
		}
	}

	if (cfg.FreeFall != nil) && (cfg.FreeFall.InterruptPin != "") {
		interruptMap, err = addInterruptPin(b, cfg.SingleTap.InterruptPin, interruptMap)
		if err != nil {
			return nil, err
		}
	}

	for _, interrupt := range interruptMap {
		ticksChan := make(chan board.Tick)
		sensor.startInterruptMonitoring(interrupt, ticksChan)
	}

	return sensor, nil
}

func (adxl *adxl345) startInterruptMonitoring(interrupt board.DigitalInterrupt, ticksChan chan board.Tick) {
	utils.PanicCapturingGo(func() {
		interrupt.AddCallback(ticksChan)
		defer interrupt.RemoveCallback(ticksChan)

		for {
			select {
			case <-adxl.cancelContext.Done():
				return
			case tick := <-ticksChan:
				if tick.High {
					func() {
						adxl.readInterrupts(adxl.cancelContext)
					}()
				}
			}
		}
	})
}

func addInterruptPin(b board.Board, interruptName string, interruptMap map[string]board.DigitalInterrupt) (map[string]board.DigitalInterrupt, error) {
	_, ok := interruptMap[interruptName]
	if !ok {
		interrupt, ok := b.DigitalInterruptByName(interruptName)
		if !ok {
			return nil, errors.Errorf("adxl345: cannot grab digital interrupt: %s", interruptName)
		}
		interruptMap[interruptName] = interrupt
	}
	return interruptMap, nil
}

func getInterruptConfigurations(cfg *AttrConfig) map[byte]byte {
	var intEnabled byte
	var intMap byte

	if cfg.FreeFall != nil {
		intEnabled += interruptBitPosition[FreeFall]
		intMap += interruptBitPosition[FreeFall] & byte(cfg.FreeFall.AccelerometerPin<<2)
	}
	if cfg.SingleTap != nil {
		intEnabled += interruptBitPosition[SingleTap]
		intMap += interruptBitPosition[SingleTap] & byte(cfg.SingleTap.AccelerometerPin<<6)
	}

	return map[byte]byte{IntEnableAddr: intEnabled, IntMapAddr: intMap}
}

// This returns a map from register addresses to data which should be written to that register to configure the interrupt pin.
func getSingleTapRegisterValues(singleTapConfigs *TapAttrConfig, registerValues map[byte]byte) map[byte]byte {
	if singleTapConfigs == nil {
		return registerValues
	}

	registerValues[TapAxesAddr] = getAxes(singleTapConfigs.ExcludeX, singleTapConfigs.ExcludeY, singleTapConfigs.ExcludeZ)

	if singleTapConfigs.Threshold != 0 {
		registerValues[ThreshTapAddr] = byte((singleTapConfigs.Threshold / 62.5))
	}
	if singleTapConfigs.Dur != 0 {
		registerValues[DurAddr] = byte((singleTapConfigs.Dur / 625))
	}
	return registerValues
}

// This returns a map from register addresses to data which should be written to that register to configure the interrupt pin.
func getFreeFallRegisterValues(freeFallConfigs *FreeFallAttrConfig, registerValues map[byte]byte) map[byte]byte {
	if freeFallConfigs == nil {
		return registerValues
	}
	if freeFallConfigs.Threshold != 0 {
		registerValues[ThreshFfAddr] = byte((freeFallConfigs.Threshold / 62.5))
	}
	if freeFallConfigs.Time != 0 {
		registerValues[TimeFfAddr] = byte((freeFallConfigs.Time / .5))
	}
	return registerValues
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

func (adxl *adxl345) configureInterruptRegisters(ctx context.Context, interruptBitMap byte) error {
	if adxl.interruptsEnabled == 0 {
		return nil
	}
	adxl.configuredRegisterValues[IntEnableAddr] = adxl.interruptsEnabled
	adxl.configuredRegisterValues[IntMapAddr] = interruptBitMap
	for key, value := range defaultRegisterValues {
		if configuredVal, ok := adxl.configuredRegisterValues[key]; ok {
			value = configuredVal
		}
		err := adxl.writeByte(ctx, key, value)
		if err != nil {
			return err
		}
	}
	return nil
}

func (adxl *adxl345) readInterrupts(ctx context.Context) {
	adxl.mu.Lock()
	defer adxl.mu.Unlock()
	intSourceRegister, err := adxl.readByte(ctx, IntSourceAddr)
	if err != nil {
		adxl.logger.Error(err)
	}

	for key, value := range interruptBitPosition {
		if intSourceRegister&value&adxl.interruptsEnabled != 0 {
			_, ok := adxl.interruptsFound[key]
			if ok {
				adxl.interruptsFound[key]++
			} else {
				adxl.interruptsFound[key] = 1
			}
		}
	}
}

// Given a value, scales it so that the range of values read in becomes the range of +/- maxValue.
// The trick here is that although the values are stored in 16 bits, the sensor only has 10 bits of
// resolution. So, there are only (1 << 9) possible positive values, and a similar number of
// negative ones.
func setScale(value int, maxValue float64) float64 {
	return float64(value) * maxValue / (1 << 9)
}

func toLinearAcceleration(data []byte) r3.Vector {
	// Vectors take ints, but we've got int16's, so we need to convert.
	x := int(rutils.Int16FromBytesLE(data[0:2]))
	y := int(rutils.Int16FromBytesLE(data[2:4]))
	z := int(rutils.Int16FromBytesLE(data[4:6]))

	// The default scale is +/- 2G's, but our units should be mm/sec/sec.
	maxAcceleration := 2.0 * 9.81 /* m/sec/sec */ * 1000.0 /* mm/m */
	return r3.Vector{
		X: setScale(x, maxAcceleration),
		Y: setScale(y, maxAcceleration),
		Z: setScale(z, maxAcceleration),
	}
}

func (adxl *adxl345) AngularVelocity(ctx context.Context, extra map[string]interface{}) (spatialmath.AngularVelocity, error) {
	return spatialmath.AngularVelocity{}, movementsensor.ErrMethodUnimplementedAngularVelocity
}

func (adxl *adxl345) LinearVelocity(ctx context.Context, extra map[string]interface{}) (r3.Vector, error) {
	return r3.Vector{}, movementsensor.ErrMethodUnimplementedLinearVelocity
}

func (adxl *adxl345) LinearAcceleration(ctx context.Context, extra map[string]interface{}) (r3.Vector, error) {
	adxl.mu.Lock()
	defer adxl.mu.Unlock()
	lastError := adxl.err.Get()

	if lastError != nil {
		return r3.Vector{}, lastError
	}

	return adxl.linearAcceleration, nil
}

func (adxl *adxl345) Orientation(ctx context.Context, extra map[string]interface{}) (spatialmath.Orientation, error) {
	return spatialmath.NewOrientationVector(), movementsensor.ErrMethodUnimplementedOrientation
}

func (adxl *adxl345) CompassHeading(ctx context.Context, extra map[string]interface{}) (float64, error) {
	return 0, movementsensor.ErrMethodUnimplementedCompassHeading
}

func (adxl *adxl345) Position(ctx context.Context, extra map[string]interface{}) (*geo.Point, float64, error) {
	return geo.NewPoint(0, 0), 0, movementsensor.ErrMethodUnimplementedPosition
}

func (adxl *adxl345) Accuracy(ctx context.Context, extra map[string]interface{}) (map[string]float32, error) {
	return map[string]float32{}, movementsensor.ErrMethodUnimplementedAccuracy
}

func (adxl *adxl345) Readings(ctx context.Context, extra map[string]interface{}) (map[string]interface{}, error) {
	adxl.mu.Lock()
	defer adxl.mu.Unlock()

	readings := make(map[string]interface{})
	readings["linear_acceleration"] = adxl.linearAcceleration
	readings["single_tap_count"] = adxl.interruptsFound[SingleTap]
	readings["freefall_count"] = adxl.interruptsFound[FreeFall]

	return readings, adxl.err.Get()
}

func (adxl *adxl345) Properties(ctx context.Context, extra map[string]interface{}) (*movementsensor.Properties, error) {
	// We don't implement any of the MovementSensor interface yet, though hopefully
	// LinearAcceleration will be added to the interface soon.
	return &movementsensor.Properties{
		LinearAccelerationSupported: true,
	}, nil
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
