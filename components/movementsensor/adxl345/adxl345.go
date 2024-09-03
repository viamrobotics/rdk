//go:build linux

// Package adxl345 implements the MovementSensor interface for the ADXL345 accelerometer.
package adxl345

/*
	This package supports ADXL345 accelerometer attached to an I2C bus on the robot (the chip supports
	communicating over SPI as well, but this package does not support that interface).
	The datasheet for this chip is available at:
	https://www.analog.com/media/en/technical-documentation/data-sheets/adxl345.pdf

	Because we only support I2C interaction, the CS pin must be wired to hot (which tells the chip
	which communication interface to use). The chip has two possible I2C addresses, which can be
	selected by wiring the SDO pin to either hot or ground:
	- if SDO is wired to ground, it uses the default I2C address of 0x53
	- if SDO is wired to hot, it uses the alternate I2C address of 0x1D

	If you use the alternate address, your config file for this component must set its
	"use_alternate_i2c_address" boolean to true.
*/

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/golang/geo/r3"
	geo "github.com/kellydunn/golang-geo"
	"github.com/pkg/errors"
	"go.viam.com/utils"

	"go.viam.com/rdk/components/board"
	"go.viam.com/rdk/components/board/genericlinux/buses"
	"go.viam.com/rdk/components/movementsensor"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/spatialmath"
	rutils "go.viam.com/rdk/utils"
)

var model = resource.DefaultModelFamily.WithModel("accel-adxl345")

const (
	defaultI2CAddress    = 0x53
	alternateI2CAddress  = 0x1D
	deviceIDRegister     = 0
	expectedDeviceID     = 0xE5
	powerControlRegister = 0x2D
)

// Config is a description of how to find an ADXL345 accelerometer on the robot.
type Config struct {
	I2cBus                 string          `json:"i2c_bus"`
	UseAlternateI2CAddress bool            `json:"use_alternate_i2c_address,omitempty"`
	BoardName              string          `json:"board,omitempty"`
	SingleTap              *TapConfig      `json:"tap,omitempty"`
	FreeFall               *FreeFallConfig `json:"free_fall,omitempty"`
}

// TapConfig is a description of the configs for tap registers.
type TapConfig struct {
	AccelerometerPin int     `json:"accelerometer_pin"`
	InterruptPin     string  `json:"interrupt_pin"`
	ExcludeX         bool    `json:"exclude_x,omitempty"`
	ExcludeY         bool    `json:"exclude_y,omitempty"`
	ExcludeZ         bool    `json:"exclude_z,omitempty"`
	Threshold        float32 `json:"threshold,omitempty"`
	Dur              float32 `json:"dur_us,omitempty"`
}

// FreeFallConfig is a description of the configs for free fall registers.
type FreeFallConfig struct {
	AccelerometerPin int     `json:"accelerometer_pin"`
	InterruptPin     string  `json:"interrupt_pin"`
	Threshold        float32 `json:"threshold,omitempty"`
	Time             float32 `json:"time_ms,omitempty"`
}

// validateTapConfigs validates the tap piece of the config.
func (tapCfg *TapConfig) validateTapConfigs() error {
	if tapCfg.AccelerometerPin != 1 && tapCfg.AccelerometerPin != 2 {
		return errors.New("Accelerometer pin on the ADXL345 must be 1 or 2")
	}
	if tapCfg.Threshold != 0 {
		if tapCfg.Threshold < 0 || tapCfg.Threshold > (255*threshTapScaleFactor) {
			return errors.New("Tap threshold on the ADXL345 must be 0 between and 15,937mg")
		}
	}
	if tapCfg.Dur != 0 {
		if tapCfg.Dur < 0 || tapCfg.Dur > (255*durScaleFactor) {
			return errors.New("Tap dur on the ADXL345 must be between 0 and 160,000µs")
		}
	}
	return nil
}

// validateFreeFallConfigs validates the freefall piece of the config.
func (freefallCfg *FreeFallConfig) validateFreeFallConfigs() error {
	if freefallCfg.AccelerometerPin != 1 && freefallCfg.AccelerometerPin != 2 {
		return errors.New("Accelerometer pin on the ADXL345 must be 1 or 2")
	}
	if freefallCfg.Threshold != 0 {
		if freefallCfg.Threshold < 0 || freefallCfg.Threshold > (255*threshFfScaleFactor) {
			return errors.New("Accelerometer tap threshold on the ADXL345 must be 0 between and 15,937mg")
		}
	}
	if freefallCfg.Time != 0 {
		if freefallCfg.Time < 0 || freefallCfg.Time > (255*timeFfScaleFactor) {
			return errors.New("Accelerometer tap time on the ADXL345 must be between 0 and 1,275ms")
		}
	}
	return nil
}

// Validate ensures all parts of the config are valid, and then returns the list of things we
// depend on.
func (cfg *Config) Validate(path string) ([]string, error) {
	var deps []string
	if cfg.BoardName == "" {
		// The board name is only required for interrupt-related functionality.
		if cfg.SingleTap != nil || cfg.FreeFall != nil {
			return nil, resource.NewConfigValidationFieldRequiredError(path, "board")
		}
	} else {
		if cfg.SingleTap != nil || cfg.FreeFall != nil {
			// The board is actually used! Add it to the dependencies.
			deps = append(deps, cfg.BoardName)
		}
	}
	if cfg.I2cBus == "" {
		return nil, resource.NewConfigValidationFieldRequiredError(path, "i2c_bus")
	}
	if cfg.SingleTap != nil {
		if err := cfg.SingleTap.validateTapConfigs(); err != nil {
			return nil, err
		}
	}
	if cfg.FreeFall != nil {
		if err := cfg.FreeFall.validateFreeFallConfigs(); err != nil {
			return nil, err
		}
	}
	return deps, nil
}

func init() {
	resource.RegisterComponent(
		movementsensor.API,
		model,
		resource.Registration[movementsensor.MovementSensor, *Config]{
			Constructor: newAdxl345,
		})
}

type adxl345 struct {
	resource.Named
	resource.AlwaysRebuild

	bus                      buses.I2C
	i2cAddress               byte
	logger                   logging.Logger
	interruptsEnabled        byte
	interruptsFound          map[InterruptID]int
	configuredRegisterValues map[byte]byte

	// Lock the mutex when you want to read or write either the acceleration or the last error.
	mu                 sync.Mutex
	linearAcceleration r3.Vector
	err                movementsensor.LastError

	workers *utils.StoppableWorkers
}

// newAdxl345 is a constructor to create a new object representing an ADXL345 accelerometer.
func newAdxl345(
	ctx context.Context,
	deps resource.Dependencies,
	conf resource.Config,
	logger logging.Logger,
) (movementsensor.MovementSensor, error) {
	newConf, err := resource.NativeConfig[*Config](conf)
	if err != nil {
		return nil, err
	}

	bus, err := buses.NewI2cBus(newConf.I2cBus)
	if err != nil {
		msg := fmt.Sprintf("can't find I2C bus '%q' for ADXL345 sensor", newConf.I2cBus)
		return nil, errors.Wrap(err, msg)
	}

	// The rest of the constructor is separated out so that you can pass in a mock I2C bus during
	// tests.
	return makeAdxl345(ctx, deps, conf, logger, bus)
}

// makeAdxl345 is split out solely to be used during unit tests: it constructs a new object
// representing an AXDL345 accelerometer, but with the I2C bus already created and passed in as an
// argument. This lets you inject a mock I2C bus during the tests. It should not be used directly
// in production code (instead, use NewAdxl345, above).
func makeAdxl345(
	ctx context.Context,
	deps resource.Dependencies,
	conf resource.Config,
	logger logging.Logger,
	bus buses.I2C,
) (movementsensor.MovementSensor, error) {
	newConf, err := resource.NativeConfig[*Config](conf)
	if err != nil {
		return nil, err
	}

	var address byte
	if newConf.UseAlternateI2CAddress {
		address = alternateI2CAddress
	} else {
		address = defaultI2CAddress
	}

	interruptConfigurations := getInterruptConfigurations(newConf)
	configuredRegisterValues := getFreeFallRegisterValues(newConf.FreeFall)
	for k, v := range getSingleTapRegisterValues(newConf.SingleTap, logger) {
		configuredRegisterValues[k] = v
	}

	sensor := &adxl345{
		Named:                    conf.ResourceName().AsNamed(),
		bus:                      bus,
		i2cAddress:               address,
		interruptsEnabled:        interruptConfigurations[intEnableAddr],
		logger:                   logger,
		configuredRegisterValues: configuredRegisterValues,
		interruptsFound:          make(map[InterruptID]int),

		// On overloaded boards, sometimes the I2C bus can be flaky. Only report errors if at least
		// 5 of the last 10 times we've tried interacting with the device have had problems.
		err: movementsensor.NewLastError(10, 5),
	}

	// To check that we're able to talk to the chip, we should be able to read register 0 and get
	// back the device ID (0xE5).
	deviceID, err := sensor.readByte(ctx, deviceIDRegister)
	if err != nil {
		return nil, movementsensor.AddressReadError(err, address, newConf.I2cBus)
	}
	if deviceID != expectedDeviceID {
		return nil, movementsensor.UnexpectedDeviceError(address, deviceID, sensor.Name().Name)
	}

	// The chip starts out in standby mode. Set it to measurement mode so we can get data from it.
	// To do this, we set the Power Control register (0x2D) to turn on the 8's bit.
	if err = sensor.writeByte(ctx, powerControlRegister, 0x08); err != nil {
		return nil, errors.Wrap(err, "unable to put ADXL345 into measurement mode")
	}

	// Now, turn on the background goroutine that constantly reads from the chip and stores data in
	// the object we created.
	sensor.workers = utils.NewBackgroundStoppableWorkers(func(cancelContext context.Context) {
		// Reading data a thousand times per second is probably fast enough.
		timer := time.NewTicker(time.Millisecond)
		defer timer.Stop()

		for {
			select {
			case <-cancelContext.Done():
				return
			default:
			}
			select {
			case <-timer.C:
				// The registers with data are 0x32 through 0x37: two bytes each for X, Y, and Z.
				rawData, err := sensor.readBlock(cancelContext, 0x32, 6)
				// Record the errors no matter what: if the error is nil, that's useful information
				// that will prevent errors from being returned later.
				sensor.err.Set(err)
				if err != nil {
					continue
				}

				linearAcceleration := toLinearAcceleration(rawData)
				// Only lock the mutex to write to the shared data, so other threads can read the
				// data as often as they want.
				sensor.mu.Lock()
				sensor.linearAcceleration = linearAcceleration
				sensor.mu.Unlock()
			case <-cancelContext.Done():
				return
			}
		}
	})

	// Clear out the source register upon starting the component
	if _, err := sensor.readByte(ctx, intSourceAddr); err != nil {
		// shut down goroutine reading sensor in the background
		sensor.workers.Stop()
		return nil, err
	}

	if err := sensor.configureInterruptRegisters(ctx, interruptConfigurations[intMapAddr]); err != nil {
		// shut down goroutine reading sensor in the background
		sensor.workers.Stop()
		return nil, err
	}

	interruptList := []string{}
	if (newConf.SingleTap != nil) && (newConf.SingleTap.InterruptPin != "") {
		interruptList = append(interruptList, newConf.SingleTap.InterruptPin)
	}

	if (newConf.FreeFall != nil) && (newConf.FreeFall.InterruptPin != "") {
		interruptList = append(interruptList, newConf.FreeFall.InterruptPin)
	}

	if len(interruptList) > 0 {
		b, err := board.FromDependencies(deps, newConf.BoardName)
		if err != nil {
			return nil, err
		}
		interrupts := []board.DigitalInterrupt{}
		for _, name := range interruptList {
			interrupt, err := b.DigitalInterruptByName(name)
			if err != nil {
				return nil, err
			}
			interrupts = append(interrupts, interrupt)
		}
		ticksChan := make(chan board.Tick)
		err = b.StreamTicks(sensor.workers.Context(), interrupts, ticksChan, nil)
		if err != nil {
			return nil, err
		}
		sensor.startInterruptMonitoring(ticksChan)
	}

	return sensor, nil
}

func (adxl *adxl345) startInterruptMonitoring(ticksChan chan board.Tick) {
	adxl.workers.Add(func(cancelContext context.Context) {
		for {
			select {
			case <-cancelContext.Done():
				return
			case tick := <-ticksChan:
				if tick.High {
					utils.UncheckedError(adxl.readInterrupts(cancelContext))
				}
			}
		}
	})
}

// This returns a map from register addresses to data which should be written to that register to configure the interrupt pin.
func getInterruptConfigurations(cfg *Config) map[byte]byte {
	var intEnabled byte
	var intMap byte

	if cfg.FreeFall != nil {
		intEnabled += interruptBitPosition[freeFall]
		if cfg.FreeFall.AccelerometerPin == 2 {
			intMap |= interruptBitPosition[freeFall]
		} else {
			// Clear the freefall bit in the map to send the signal to pin INT1.
			intMap &^= interruptBitPosition[freeFall]
		}
	}
	if cfg.SingleTap != nil {
		intEnabled += interruptBitPosition[singleTap]
		if cfg.SingleTap.AccelerometerPin == 2 {
			intMap |= interruptBitPosition[singleTap]
		} else {
			// Clear the single tap bit in the map to send the signal to pin INT1.
			intMap &^= interruptBitPosition[singleTap]
		}
	}

	return map[byte]byte{intEnableAddr: intEnabled, intMapAddr: intMap}
}

// This returns a map from register addresses to data which should be written to that register to configure single tap.
func getSingleTapRegisterValues(singleTapConfigs *TapConfig, logger logging.Logger) map[byte]byte {
	registerValues := map[byte]byte{}
	if singleTapConfigs == nil {
		return registerValues
	}

	registerValues[tapAxesAddr] = getAxes(singleTapConfigs.ExcludeX, singleTapConfigs.ExcludeY, singleTapConfigs.ExcludeZ)

	if singleTapConfigs.Threshold != 0 {
		registerValues[threshTapAddr] = byte((singleTapConfigs.Threshold / threshTapScaleFactor))
	}
	if singleTapConfigs.Dur != 0 {
		registerValues[durAddr] = byte((singleTapConfigs.Dur / durScaleFactor))
	}

	logger.Info("Consider experimenting with dur_us and threshold attributes to achieve best results with single tap")
	return registerValues
}

// This returns a map from register addresses to data which should be written to that register to configure freefall.
func getFreeFallRegisterValues(freeFallConfigs *FreeFallConfig) map[byte]byte {
	registerValues := map[byte]byte{}
	if freeFallConfigs == nil {
		return registerValues
	}
	if freeFallConfigs.Threshold != 0 {
		registerValues[threshFfAddr] = byte((freeFallConfigs.Threshold / threshFfScaleFactor))
	}
	if freeFallConfigs.Time != 0 {
		registerValues[timeFfAddr] = byte((freeFallConfigs.Time / timeFfScaleFactor))
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
			adxl.logger.CError(ctx, err)
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
			adxl.logger.CError(ctx, err)
		}
	}()

	return handle.WriteByteData(ctx, register, value)
}

func (adxl *adxl345) configureInterruptRegisters(ctx context.Context, interruptBitMap byte) error {
	if adxl.interruptsEnabled == 0 {
		return nil
	}
	adxl.configuredRegisterValues[intEnableAddr] = adxl.interruptsEnabled
	adxl.configuredRegisterValues[intMapAddr] = interruptBitMap
	for key, value := range defaultRegisterValues {
		if configuredVal, ok := adxl.configuredRegisterValues[key]; ok {
			value = configuredVal
		}
		if err := adxl.writeByte(ctx, key, value); err != nil {
			return err
		}
	}
	return nil
}

func (adxl *adxl345) readInterrupts(ctx context.Context) error {
	adxl.mu.Lock()
	defer adxl.mu.Unlock()
	intSourceRegister, err := adxl.readByte(ctx, intSourceAddr)
	if err != nil {
		return err
	}

	for key, val := range interruptBitPosition {
		if intSourceRegister&val&adxl.interruptsEnabled != 0 {
			adxl.interruptsFound[key]++
		}
	}
	return nil
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

	// The default scale is +/- 2G's, but our units should be m/sec/sec.
	maxAcceleration := 2.0 * 9.81 /* m/sec/sec */
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

func (adxl *adxl345) Accuracy(ctx context.Context, extra map[string]interface{}) (*movementsensor.Accuracy, error) {
	// this driver is unable to provide positional or compass heading data
	return movementsensor.UnimplementedOptionalAccuracies(), nil
}

func (adxl *adxl345) Readings(ctx context.Context, extra map[string]interface{}) (map[string]interface{}, error) {
	readings, err := movementsensor.DefaultAPIReadings(ctx, adxl, extra)
	if err != nil {
		return nil, err
	}

	adxl.mu.Lock()
	defer adxl.mu.Unlock()

	readings["single_tap_count"] = adxl.interruptsFound[singleTap]
	readings["freefall_count"] = adxl.interruptsFound[freeFall]

	return readings, adxl.err.Get()
}

func (adxl *adxl345) Properties(ctx context.Context, extra map[string]interface{}) (*movementsensor.Properties, error) {
	return &movementsensor.Properties{
		LinearAccelerationSupported: true,
	}, nil
}

// Puts the chip into standby mode.
func (adxl *adxl345) Close(ctx context.Context) error {
	adxl.workers.Stop()

	adxl.mu.Lock()
	defer adxl.mu.Unlock()

	// Put the chip into standby mode by setting the Power Control register (0x2D) to 0.
	err := adxl.writeByte(ctx, powerControlRegister, 0x00)
	if err != nil {
		adxl.logger.CErrorf(ctx, "unable to turn off ADXL345 accelerometer: '%s'", err)
	}
	return nil
}
