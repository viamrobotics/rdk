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
	"sync"
	"time"

	"github.com/golang/geo/r3"
	geo "github.com/kellydunn/golang-geo"
	"github.com/pkg/errors"
	"go.viam.com/utils"

	"go.viam.com/rdk/components/board"
	"go.viam.com/rdk/components/movementsensor"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/spatialmath"
	rutils "go.viam.com/rdk/utils"
)

var model = resource.DefaultModelFamily.WithModel("accel-adxl345")

const (
	deviceIDRegister     = 0
	expectedDeviceID     = 0xE5
	powerControlRegister = 0x2D
)

// Config is a description of how to find an ADXL345 accelerometer on the robot.
type Config struct {
	BoardName              string          `json:"board"`
	I2cBus                 string          `json:"i2c_bus"`
	UseAlternateI2CAddress bool            `json:"use_alternate_i2c_address,omitempty"`
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

// ValidateTapConfigs validates the tap piece of the config.
func (tapCfg *TapConfig) ValidateTapConfigs(path string) error {
	if tapCfg.AccelerometerPin != 1 && tapCfg.AccelerometerPin != 2 {
		return errors.New("Accelerometer pin on the ADXL345 must be 1 or 2")
	}
	if tapCfg.Threshold != 0 {
		if tapCfg.Threshold < 0 || tapCfg.Threshold > (255*ThreshTapScaleFactor) {
			return errors.New("Tap threshold on the ADXL345 must be 0 between and 15,937mg")
		}
	}
	if tapCfg.Dur != 0 {
		if tapCfg.Dur < 0 || tapCfg.Dur > (255*DurScaleFactor) {
			return errors.New("Tap dur on the ADXL345 must be between 0 and 160,000µs")
		}
	}
	return nil
}

// ValidateFreeFallConfigs validates the freefall piece of the config.
func (freefallCfg *FreeFallConfig) ValidateFreeFallConfigs(path string) error {
	if freefallCfg.AccelerometerPin != 1 && freefallCfg.AccelerometerPin != 2 {
		return errors.New("Accelerometer pin on the ADXL345 must be 1 or 2")
	}
	if freefallCfg.Threshold != 0 {
		if freefallCfg.Threshold < 0 || freefallCfg.Threshold > (255*ThreshFfScaleFactor) {
			return errors.New("Accelerometer tap threshold on the ADXL345 must be 0 between and 15,937mg")
		}
	}
	if freefallCfg.Time != 0 {
		if freefallCfg.Time < 0 || freefallCfg.Time > (255*TimeFfScaleFactor) {
			return errors.New("Accelerometer tap time on the ADXL345 must be between 0 and 1,275ms")
		}
	}
	return nil
}

// Validate ensures all parts of the config are valid, and then returns the list of things we
// depend on.
func (cfg *Config) Validate(path string) ([]string, error) {
	if cfg.BoardName == "" {
		return nil, utils.NewConfigValidationFieldRequiredError(path, "board")
	}
	if cfg.I2cBus == "" {
		return nil, utils.NewConfigValidationFieldRequiredError(path, "i2c_bus")
	}
	if cfg.SingleTap != nil {
		if err := cfg.SingleTap.ValidateTapConfigs(path); err != nil {
			return nil, err
		}
	}
	if cfg.FreeFall != nil {
		if err := cfg.FreeFall.ValidateFreeFallConfigs(path); err != nil {
			return nil, err
		}
	}
	var deps []string
	deps = append(deps, cfg.BoardName)
	return deps, nil
}

func init() {
	resource.RegisterComponent(
		movementsensor.API,
		model,
		resource.Registration[movementsensor.MovementSensor, *Config]{
			Constructor: NewAdxl345,
		})
}

type adxl345 struct {
	resource.Named
	resource.AlwaysRebuild

	bus                      board.I2C
	i2cAddress               byte
	logger                   logging.Logger
	interruptsEnabled        byte
	interruptsFound          map[InterruptID]int
	configuredRegisterValues map[byte]byte

	// Used only to remove the callbacks from the interrupts upon closing component.
	interruptChannels map[board.DigitalInterrupt]chan board.Tick

	// Lock the mutex when you want to read or write either the acceleration or the last error.
	mu                 sync.Mutex
	linearAcceleration r3.Vector
	err                movementsensor.LastError

	// Used to shut down the background goroutine which polls the sensor.
	cancelContext           context.Context
	cancelFunc              func()
	activeBackgroundWorkers sync.WaitGroup
}

// NewAdxl345 is a constructor to create a new object representing an ADXL345 accelerometer.
func NewAdxl345(
	ctx context.Context,
	deps resource.Dependencies,
	conf resource.Config,
	logger logging.Logger,
) (movementsensor.MovementSensor, error) {
	newConf, err := resource.NativeConfig[*Config](conf)
	if err != nil {
		return nil, err
	}
	b, err := board.FromDependencies(deps, newConf.BoardName)
	if err != nil {
		return nil, err
	}
	localB, ok := b.(board.LocalBoard)
	if !ok {
		return nil, errors.Errorf("board %q is not local", newConf.BoardName)
	}
	bus, ok := localB.I2CByName(newConf.I2cBus)
	if !ok {
		return nil, errors.Errorf("can't find I2C bus '%q' for ADXL345 sensor", newConf.I2cBus)
	}

	var address byte
	if newConf.UseAlternateI2CAddress {
		address = 0x1D
	} else {
		address = 0x53
	}

	interruptConfigurations := getInterruptConfigurations(newConf)
	configuredRegisterValues := getFreeFallRegisterValues(newConf.FreeFall)
	for k, v := range getSingleTapRegisterValues(newConf.SingleTap) {
		configuredRegisterValues[k] = v
	}
	cancelContext, cancelFunc := context.WithCancel(context.Background())

	sensor := &adxl345{
		Named:                    conf.ResourceName().AsNamed(),
		bus:                      bus,
		i2cAddress:               address,
		interruptsEnabled:        interruptConfigurations[IntEnableAddr],
		logger:                   logger,
		cancelContext:            cancelContext,
		cancelFunc:               cancelFunc,
		configuredRegisterValues: configuredRegisterValues,
		interruptsFound:          make(map[InterruptID]int),
		interruptChannels:        make(map[board.DigitalInterrupt]chan board.Tick),

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
	sensor.activeBackgroundWorkers.Add(1)
	utils.PanicCapturingGo(func() {
		defer sensor.activeBackgroundWorkers.Done()
		// Reading data a thousand times per second is probably fast enough.
		timer := time.NewTicker(time.Millisecond)
		defer timer.Stop()

		for {
			select {
			case <-sensor.cancelContext.Done():
				return
			default:
			}
			select {
			case <-timer.C:
				// The registers with data are 0x32 through 0x37: two bytes each for X, Y, and Z.
				rawData, err := sensor.readBlock(sensor.cancelContext, 0x32, 6)
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
			case <-sensor.cancelContext.Done():
				return
			}
		}
	})

	// Clear out the source register upon starting the component
	if _, err := sensor.readByte(ctx, IntSourceAddr); err != nil {
		// shut down goroutine reading sensor in the background
		sensor.cancelFunc()
		return nil, err
	}

	if err := sensor.configureInterruptRegisters(ctx, interruptConfigurations[IntMapAddr]); err != nil {
		// shut down goroutine reading sensor in the background
		sensor.cancelFunc()
		return nil, err
	}

	interruptMap := map[string]board.DigitalInterrupt{}
	if (newConf.SingleTap != nil) && (newConf.SingleTap.InterruptPin != "") {
		interruptMap, err = addInterruptPin(b, newConf.SingleTap.InterruptPin, interruptMap)
		if err != nil {
			// shut down goroutine reading sensor in the background
			sensor.cancelFunc()
			return nil, err
		}
	}

	if (newConf.FreeFall != nil) && (newConf.FreeFall.InterruptPin != "") {
		interruptMap, err = addInterruptPin(b, newConf.FreeFall.InterruptPin, interruptMap)
		if err != nil {
			// shut down goroutine reading sensor in the background
			sensor.cancelFunc()
			return nil, err
		}
	}

	for _, interrupt := range interruptMap {
		ticksChan := make(chan board.Tick)
		interrupt.AddCallback(ticksChan)
		sensor.interruptChannels[interrupt] = ticksChan
		sensor.startInterruptMonitoring(ticksChan)
	}

	return sensor, nil
}

func (adxl *adxl345) startInterruptMonitoring(ticksChan chan board.Tick) {
	utils.PanicCapturingGo(func() {
		for {
			select {
			case <-adxl.cancelContext.Done():
				return
			case tick := <-ticksChan:
				if tick.High {
					utils.UncheckedError(adxl.readInterrupts(adxl.cancelContext))
				}
			}
		}
	})
}

func addInterruptPin(b board.Board, name string, interrupts map[string]board.DigitalInterrupt) (map[string]board.DigitalInterrupt, error) {
	_, ok := interrupts[name]
	if !ok {
		interrupt, ok := b.DigitalInterruptByName(name)
		if !ok {
			return nil, errors.Errorf("cannot grab digital interrupt: %s", name)
		}
		interrupts[name] = interrupt
	}
	return interrupts, nil
}

// This returns a map from register addresses to data which should be written to that register to configure the interrupt pin.
func getInterruptConfigurations(cfg *Config) map[byte]byte {
	var intEnabled byte
	var intMap byte

	if cfg.FreeFall != nil {
		intEnabled += interruptBitPosition[FreeFall]
		if cfg.FreeFall.AccelerometerPin == 2 {
			intMap |= interruptBitPosition[FreeFall]
		} else {
			// Clear the freefall bit in the map to send the signal to pin INT1.
			intMap &^= interruptBitPosition[FreeFall]
		}
	}
	if cfg.SingleTap != nil {
		intEnabled += interruptBitPosition[SingleTap]
		if cfg.SingleTap.AccelerometerPin == 2 {
			intMap |= interruptBitPosition[SingleTap]
		} else {
			// Clear the single tap bit in the map to send the signal to pin INT1.
			intMap &^= interruptBitPosition[SingleTap]
		}
	}

	return map[byte]byte{IntEnableAddr: intEnabled, IntMapAddr: intMap}
}

// This returns a map from register addresses to data which should be written to that register to configure single tap.
func getSingleTapRegisterValues(singleTapConfigs *TapConfig) map[byte]byte {
	registerValues := map[byte]byte{}
	if singleTapConfigs == nil {
		return registerValues
	}

	registerValues[TapAxesAddr] = getAxes(singleTapConfigs.ExcludeX, singleTapConfigs.ExcludeY, singleTapConfigs.ExcludeZ)

	if singleTapConfigs.Threshold != 0 {
		registerValues[ThreshTapAddr] = byte((singleTapConfigs.Threshold / ThreshTapScaleFactor))
	}
	if singleTapConfigs.Dur != 0 {
		registerValues[DurAddr] = byte((singleTapConfigs.Dur / DurScaleFactor))
	}
	return registerValues
}

// This returns a map from register addresses to data which should be written to that register to configure freefall.
func getFreeFallRegisterValues(freeFallConfigs *FreeFallConfig) map[byte]byte {
	registerValues := map[byte]byte{}
	if freeFallConfigs == nil {
		return registerValues
	}
	if freeFallConfigs.Threshold != 0 {
		registerValues[ThreshFfAddr] = byte((freeFallConfigs.Threshold / ThreshFfScaleFactor))
	}
	if freeFallConfigs.Time != 0 {
		registerValues[TimeFfAddr] = byte((freeFallConfigs.Time / TimeFfScaleFactor))
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
		if err := adxl.writeByte(ctx, key, value); err != nil {
			return err
		}
	}
	return nil
}

func (adxl *adxl345) readInterrupts(ctx context.Context) error {
	adxl.mu.Lock()
	defer adxl.mu.Unlock()
	intSourceRegister, err := adxl.readByte(ctx, IntSourceAddr)
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

func (adxl *adxl345) Accuracy(ctx context.Context, extra map[string]interface{}) (map[string]float32, error) {
	return map[string]float32{}, movementsensor.ErrMethodUnimplementedAccuracy
}

func (adxl *adxl345) Readings(ctx context.Context, extra map[string]interface{}) (map[string]interface{}, error) {
	readings, err := movementsensor.DefaultAPIReadings(ctx, adxl, extra)
	if err != nil {
		return nil, err
	}

	adxl.mu.Lock()
	defer adxl.mu.Unlock()

	readings["single_tap_count"] = adxl.interruptsFound[SingleTap]
	readings["freefall_count"] = adxl.interruptsFound[FreeFall]

	return readings, adxl.err.Get()
}

func (adxl *adxl345) Properties(ctx context.Context, extra map[string]interface{}) (*movementsensor.Properties, error) {
	return &movementsensor.Properties{
		LinearAccelerationSupported: true,
	}, nil
}

// Puts the chip into standby mode.
func (adxl *adxl345) Close(ctx context.Context) error {
	adxl.cancelFunc()
	adxl.activeBackgroundWorkers.Wait()

	adxl.mu.Lock()
	defer adxl.mu.Unlock()

	for interrupt, channel := range adxl.interruptChannels {
		interrupt.RemoveCallback(channel)
	}

	// Put the chip into standby mode by setting the Power Control register (0x2D) to 0.
	err := adxl.writeByte(ctx, powerControlRegister, 0x00)
	if err != nil {
		adxl.logger.Errorf("unable to turn off ADXL345 accelerometer: '%s'", err)
	}
	return nil
}
