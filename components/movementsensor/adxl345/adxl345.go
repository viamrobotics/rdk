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
	BoardName              string                 `json:"board"`
	I2cBus                 string                 `json:"i2c_bus"`
	UseAlternateI2CAddress bool                   `json:"use_alternate_i2c_address,omitempty"`
	InterruptPin1          InterruptPinAttrConfig `json:"interrupt_pin1,omitempty"`
	InterruptPin2          InterruptPinAttrConfig `json:"interrupt_pin2,omitempty"`
}

type InterruptPinAttrConfig struct {
	EchoInterrupt      string              `json:"echo_interrupt_pin"`
	SingleTap          bool                `json:"single_tap,omitempty"`
	FreeFall           bool                `json:"free_fall,omitempty"`
	TapAttrConfig      *TapAttrConfig      `json:"single_tap_attributes,omitempty"`
	FreeFallAttrConfig *FreeFallAttrConfig `json:"free_fall_attributes,omitempty"`
}

type TapAttrConfig struct {
	ExcludeTapX bool `json:"tap_x,omitempty"`
	ExcludeTapY bool `json:"tap_y,omitempty"`
	ExcludeTapZ bool `json:"tap_z,omitempty"`
	ThreshTap   byte `json:"thresh_tap,omitempty"`
	Dur         byte `json:"dur,omitempty"`
}

type FreeFallAttrConfig struct {
	EchoInterrupt string `json:"echo_interrupt_pin"`
	ThreshFF      byte   `json:"thresh_ff,omitempty"`
	TimeFF        byte   `json:"time_ff,omitempty"`
}

func (cfg *InterruptPinAttrConfig) Validate(path string) ([]string, error) {
	if cfg.EchoInterrupt == "" {
		return nil, utils.NewConfigValidationFieldRequiredError(path, "echo_interrupt_pin")
	}

	if cfg.SingleTap {
		if cfg.TapAttrConfig == nil {
			return nil, utils.NewConfigValidationFieldRequiredError(path, "single_tap_attributes")
		}
	}
	if cfg.FreeFall {
		if cfg.FreeFallAttrConfig == nil {
			return nil, utils.NewConfigValidationFieldRequiredError(path, "free_fall_attributes")
		}
	}
	var deps []string
	deps = append(deps, cfg.EchoInterrupt)
	return deps, nil

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
	echoInterrupt1           board.DigitalInterrupt
	echoInterrupt2           board.DigitalInterrupt
	interruptsEnabled        byte
	interruptsMap            byte
	ticksChan                chan board.Tick
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
	interruptConfigurations := getInterruptConfigurations(cfg.InterruptPin1, cfg.InterruptPin2)
	configuredRegisterValues := getAllRegisterValues(cfg.InterruptPin1, cfg.InterruptPin2)
	cancelContext, cancelFunc := context.WithCancel(ctx)
	sensor := &adxl345{
		bus:                      bus,
		i2cAddress:               address,
		interruptsEnabled:        interruptConfigurations[IntEnableAddr],
		interruptsMap:            interruptConfigurations[IntMapAddr],
		logger:                   logger,
		cancelContext:            cancelContext,
		cancelFunc:               cancelFunc,
		configuredRegisterValues: configuredRegisterValues,
	}
	if cfg.InterruptPin1.EchoInterrupt != "" {
		i1, ok := b.DigitalInterruptByName(cfg.InterruptPin1.EchoInterrupt)
		if !ok {
			return nil, errors.Errorf("adxl345: cannot grab digital interrupt: %s", cfg.InterruptPin1.EchoInterrupt)
		}
		sensor.echoInterrupt1 = i1
	}
	if cfg.InterruptPin2.EchoInterrupt != "" {
		i2, ok := b.DigitalInterruptByName(cfg.InterruptPin2.EchoInterrupt)
		if !ok {
			return nil, errors.Errorf("adxl345: cannot grab digital interrupt: %s", cfg.InterruptPin2.EchoInterrupt)
		}
		sensor.echoInterrupt1 = i2
	}
	sensor.ticksChan = make(chan board.Tick)

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
	err = sensor.configureInterruptRegisters(ctx)
	if err != nil {
		return nil, err
	}
	if sensor.echoInterrupt1 != nil {
		sensor.startInterruptPolling(sensor.echoInterrupt1)
	}
	if sensor.echoInterrupt2 != nil {
		sensor.startInterruptPolling(sensor.echoInterrupt2)
	}
	return sensor, nil
}

func (adxl *adxl345) startInterruptPolling(interrupt board.DigitalInterrupt) {
	utils.PanicCapturingGo(func() {
		interrupt.AddCallback(adxl.ticksChan)
		defer interrupt.RemoveCallback(adxl.ticksChan)

		for {
			select {
			case <-adxl.cancelContext.Done():
				return
			case tick := <-adxl.ticksChan:
				if tick.High {
					func() {
						adxl.mu.Lock()
						defer adxl.mu.Unlock()
						adxl.readInterrupts(adxl.cancelContext)
					}()
				}
			}
		}
	})
}

func getInterruptConfigurations(int1 InterruptPinAttrConfig, int2 InterruptPinAttrConfig) map[byte]byte {
	var intEnabled byte
	var intMap byte

	if int1.SingleTap {
		intEnabled += interruptBitPosition[SingleTap]
	} else if int2.SingleTap {
		intEnabled += interruptBitPosition[SingleTap]
		intMap += interruptBitPosition[SingleTap]
	}

	if int1.FreeFall {
		intEnabled += interruptBitPosition[FreeFall]
	} else if int2.FreeFall {
		intEnabled += interruptBitPosition[FreeFall]
		intMap += interruptBitPosition[FreeFall]
	}
	return map[byte]byte{IntEnableAddr: intEnabled, IntMapAddr: intMap}
}

// This returns a map from register addresses to data which should be written to that register to configure the interrupt pin
func getSingleTapRegisterValuesFromInterruptPin(interruptPin InterruptPinAttrConfig, registerValues map[byte]byte) map[byte]byte {
	singleTapConfigs := interruptPin.TapAttrConfig
	if singleTapConfigs == nil {
		return registerValues
	}

	tapAxesSpecified := singleTapConfigs.ExcludeTapX || singleTapConfigs.ExcludeTapY || singleTapConfigs.ExcludeTapZ
	if tapAxesSpecified {
		registerValues[TapAxesAddr] = getAxes(singleTapConfigs.ExcludeTapX, singleTapConfigs.ExcludeTapY, singleTapConfigs.ExcludeTapZ)
	}
	if singleTapConfigs.ThreshTap != 0 {
		registerValues[ThreshTapAddr] = singleTapConfigs.ThreshTap
	}
	if singleTapConfigs.Dur != 0 {
		registerValues[DurAddr] = singleTapConfigs.Dur
	}
	return registerValues
}

// This returns a map from register addresses to data which should be written to that register to configure the interrupt pin
func getFreeFallRegisterValuesFromInterruptPin(interruptPin InterruptPinAttrConfig, registerValues map[byte]byte) map[byte]byte {
	freeFallConfigs := interruptPin.FreeFallAttrConfig
	if freeFallConfigs == nil {
		return registerValues
	}
	if freeFallConfigs.ThreshFF != 0 {
		registerValues[ThreshFfAddr] = freeFallConfigs.ThreshFF
	}
	if freeFallConfigs.TimeFF != 0 {
		registerValues[TimeFfAddr] = freeFallConfigs.TimeFF
	}
	return registerValues
}

func getAllRegisterValues(int1 InterruptPinAttrConfig, int2 InterruptPinAttrConfig) map[byte]byte {
	interruptRegisterValues := map[byte]byte{}

	if int1.SingleTap || int1.FreeFall {
		interruptRegisterValues = getSingleTapRegisterValuesFromInterruptPin(int1, interruptRegisterValues)
		interruptRegisterValues = getFreeFallRegisterValuesFromInterruptPin(int1, interruptRegisterValues)
	}
	if int2.SingleTap || int2.FreeFall {
		interruptRegisterValues = getSingleTapRegisterValuesFromInterruptPin(int1, interruptRegisterValues)
		interruptRegisterValues = getFreeFallRegisterValuesFromInterruptPin(int1, interruptRegisterValues)
	}
	return interruptRegisterValues
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

func (adxl *adxl345) configureInterruptRegisters(ctx context.Context) error {
	if adxl.interruptsEnabled == 0 {
		return nil
	}
	adxl.configuredRegisterValues[IntEnableAddr] = adxl.interruptsEnabled
	adxl.configuredRegisterValues[IntMapAddr] = adxl.interruptsMap
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
	intSourceRegister, err := adxl.readByte(ctx, IntSourceAddr)
	if err != nil {
		adxl.logger.Error(err)
	}

	for key, value := range interruptBitPosition {
		if intSourceRegister&value&adxl.interruptsEnabled != 0 {
			_, ok := adxl.interruptsFound[key]
			if ok {
				adxl.interruptsFound[key] += 1
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

func contains(s []string, str string) bool {
	for _, v := range s {
		if v == str {
			return true
		}
	}
	return false
}

func (adxl *adxl345) Readings(ctx context.Context, extra map[string]interface{}) (map[string]interface{}, error) {
	adxl.mu.Lock()
	defer adxl.mu.Unlock()

	readings := make(map[string]interface{})
	readings["linear_acceleration"] = adxl.linearAcceleration
	readings["single_tap"] = adxl.interruptsFound[SingleTap]
	readings["freefall"] = adxl.interruptsFound[FreeFall]

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
