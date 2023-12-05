//go:build linux

// Package mpu6050 implements the movementsensor interface for an MPU-6050 6-axis accelerometer. A
// datasheet for this chip is at
// https://components101.com/sites/default/files/component_datasheet/MPU6050-DataSheet.pdf and a
// description of the I2C registers is at
// https://download.datasheets.com/pdfs/2015/3/19/8/3/59/59/invse_/manual/5rm-mpu-6000a-00v4.2.pdf
//
// We support reading the accelerometer, gyroscope, and thermometer data off of the chip. We do not
// yet support using the digital interrupt pin to notify on events (freefall, collision, etc.),
// nor do we yet support using the secondary I2C connection to add an external clock or
// magnetometer.
//
// The chip has two possible I2C addresses, which can be selected by wiring the AD0 pin to either
// hot or ground:
//   - if AD0 is wired to ground, it uses the default I2C address of 0x68
//   - if AD0 is wired to hot, it uses the alternate I2C address of 0x69
//
// If you use the alternate address, your config file for this component must set its
// "use_alternate_i2c_address" boolean to true.
package mpu6050

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/golang/geo/r3"
	geo "github.com/kellydunn/golang-geo"
	"github.com/pkg/errors"
	"go.viam.com/utils"

	"go.viam.com/rdk/components/board/genericlinux/buses"
	"go.viam.com/rdk/components/movementsensor"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/spatialmath"
	rutils "go.viam.com/rdk/utils"
)

var model = resource.DefaultModelFamily.WithModel("gyro-mpu6050")

const (
	defaultAddressRegister = 117
	expectedDefaultAddress = 0x68
	alternateAddress       = 0x69
)

// Config is used to configure the attributes of the chip.
type Config struct {
	I2cBus                 string `json:"i2c_bus"`
	UseAlternateI2CAddress bool   `json:"use_alt_i2c_address,omitempty"`
}

// Validate ensures all parts of the config are valid, and then returns the list of things we
// depend on.
func (conf *Config) Validate(path string) ([]string, error) {
	if conf.I2cBus == "" {
		return nil, resource.NewConfigValidationFieldRequiredError(path, "i2c_bus")
	}

	var deps []string
	return deps, nil
}

func init() {
	resource.RegisterComponent(movementsensor.API, model, resource.Registration[movementsensor.MovementSensor, *Config]{
		Constructor: NewMpu6050,
	})
}

type mpu6050 struct {
	resource.Named
	resource.AlwaysRebuild
	bus        buses.I2C
	i2cAddress byte
	mu         sync.Mutex

	// The 3 things we can measure: lock the mutex before reading or writing these.
	angularVelocity    spatialmath.AngularVelocity
	temperature        float64
	linearAcceleration r3.Vector
	// Stores the most recent error from the background goroutine
	err movementsensor.LastError

	// Used to shut down the background goroutine which polls the sensor.
	backgroundContext       context.Context
	cancelFunc              func()
	activeBackgroundWorkers sync.WaitGroup
	logger                  logging.Logger
}

func addressReadError(err error, address byte, bus string) error {
	msg := fmt.Sprintf("can't read from I2C address %d on bus %s", address, bus)
	return errors.Wrap(err, msg)
}

func unexpectedDeviceError(address, defaultAddress byte) error {
	return errors.Errorf("unexpected non-MPU6050 device at address %d: response '%d'",
		address, defaultAddress)
}

// NewMpu6050 constructs a new Mpu6050 object.
func NewMpu6050(
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
		return nil, err
	}
	return makeMpu6050(ctx, deps, conf, logger, bus)
}

// This function is separated from NewMpu6050 solely so you can inject a mock I2C bus in tests.
func makeMpu6050(
	ctx context.Context,
	_ resource.Dependencies,
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
		address = alternateAddress
	} else {
		address = expectedDefaultAddress
	}
	logger.Debugf("Using address %d for MPU6050 sensor", address)

	backgroundContext, cancelFunc := context.WithCancel(context.Background())
	sensor := &mpu6050{
		Named:             conf.ResourceName().AsNamed(),
		bus:               bus,
		i2cAddress:        address,
		logger:            logger,
		backgroundContext: backgroundContext,
		cancelFunc:        cancelFunc,
		// On overloaded boards, the I2C bus can become flaky. Only report errors if at least 5 of
		// the last 10 attempts to talk to the device have failed.
		err: movementsensor.NewLastError(10, 5),
	}

	// To check that we're able to talk to the chip, we should be able to read register 117 and get
	// back the device's non-alternative address (0x68)
	defaultAddress, err := sensor.readByte(ctx, defaultAddressRegister)
	if err != nil {
		return nil, addressReadError(err, address, newConf.I2cBus)
	}
	if defaultAddress != expectedDefaultAddress {
		return nil, unexpectedDeviceError(address, defaultAddress)
	}

	// The chip starts out in standby mode (the Sleep bit in the power management register defaults
	// to 1). Set it to measurement mode (by turning off the Sleep bit) so we can get data from it.
	// To do this, we set register 107 to 0.
	err = sensor.writeByte(ctx, 107, 0)
	if err != nil {
		return nil, errors.Errorf("Unable to wake up MPU6050: '%s'", err.Error())
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
				rawData, err := sensor.readBlock(sensor.backgroundContext, 59, 14)
				// Record `err` no matter what: even if it's nil, that's useful information.
				sensor.err.Set(err)
				if err != nil {
					sensor.logger.Errorf("error reading MPU6050 sensor: '%s'", err)
					continue
				}

				linearAcceleration := toLinearAcceleration(rawData[0:6])
				// Taken straight from the MPU6050 register map. Yes, these are weird constants.
				temperature := float64(rutils.Int16FromBytesBE(rawData[6:8]))/340.0 + 36.53
				angularVelocity := toAngularVelocity(rawData[8:14])

				// Lock the mutex before modifying the state within the object. By keeping the mutex
				// unlocked for everything else, we maximize the time when another thread can read the
				// values.
				sensor.mu.Lock()
				sensor.linearAcceleration = linearAcceleration
				sensor.temperature = temperature
				sensor.angularVelocity = angularVelocity
				sensor.mu.Unlock()
			case <-sensor.backgroundContext.Done():
				return
			}
		}
	})

	return sensor, nil
}

func (mpu *mpu6050) readByte(ctx context.Context, register byte) (byte, error) {
	result, err := mpu.readBlock(ctx, register, 1)
	if err != nil {
		return 0, err
	}
	return result[0], err
}

func (mpu *mpu6050) readBlock(ctx context.Context, register byte, length uint8) ([]byte, error) {
	handle, err := mpu.bus.OpenHandle(mpu.i2cAddress)
	if err != nil {
		return nil, err
	}
	defer func() {
		err := handle.Close()
		if err != nil {
			mpu.logger.Error(err)
		}
	}()

	results, err := handle.ReadBlockData(ctx, register, length)
	return results, err
}

func (mpu *mpu6050) writeByte(ctx context.Context, register, value byte) error {
	handle, err := mpu.bus.OpenHandle(mpu.i2cAddress)
	if err != nil {
		return err
	}
	defer func() {
		err := handle.Close()
		if err != nil {
			mpu.logger.Error(err)
		}
	}()

	return handle.WriteByteData(ctx, register, value)
}

// Given a value, scales it so that the range of int16s becomes the range of +/- maxValue.
func setScale(value int, maxValue float64) float64 {
	return float64(value) * maxValue / (1 << 15)
}

// A helper function to abstract out shared code: takes 6 bytes and gives back AngularVelocity, in
// radians per second.
func toAngularVelocity(data []byte) spatialmath.AngularVelocity {
	gx := int(rutils.Int16FromBytesBE(data[0:2]))
	gy := int(rutils.Int16FromBytesBE(data[2:4]))
	gz := int(rutils.Int16FromBytesBE(data[4:6]))

	maxRotation := 250.0 // Maximum degrees per second measurable in the default configuration
	return spatialmath.AngularVelocity{
		X: setScale(gx, maxRotation),
		Y: setScale(gy, maxRotation),
		Z: setScale(gz, maxRotation),
	}
}

// A helper function that takes 6 bytes and gives back linear acceleration.
func toLinearAcceleration(data []byte) r3.Vector {
	x := int(rutils.Int16FromBytesBE(data[0:2]))
	y := int(rutils.Int16FromBytesBE(data[2:4]))
	z := int(rutils.Int16FromBytesBE(data[4:6]))

	// The scale is +/- 2G's, but our units should be m/sec/sec.
	maxAcceleration := 2.0 * 9.81 /* m/sec/sec */
	return r3.Vector{
		X: setScale(x, maxAcceleration),
		Y: setScale(y, maxAcceleration),
		Z: setScale(z, maxAcceleration),
	}
}

func (mpu *mpu6050) AngularVelocity(ctx context.Context, extra map[string]interface{}) (spatialmath.AngularVelocity, error) {
	mpu.mu.Lock()
	defer mpu.mu.Unlock()
	return mpu.angularVelocity, mpu.err.Get()
}

func (mpu *mpu6050) LinearVelocity(ctx context.Context, extra map[string]interface{}) (r3.Vector, error) {
	return r3.Vector{}, movementsensor.ErrMethodUnimplementedLinearVelocity
}

func (mpu *mpu6050) LinearAcceleration(ctx context.Context, exta map[string]interface{}) (r3.Vector, error) {
	mpu.mu.Lock()
	defer mpu.mu.Unlock()

	lastError := mpu.err.Get()
	if lastError != nil {
		return r3.Vector{}, lastError
	}
	return mpu.linearAcceleration, nil
}

func (mpu *mpu6050) Orientation(ctx context.Context, extra map[string]interface{}) (spatialmath.Orientation, error) {
	return spatialmath.NewOrientationVector(), movementsensor.ErrMethodUnimplementedOrientation
}

func (mpu *mpu6050) CompassHeading(ctx context.Context, extra map[string]interface{}) (float64, error) {
	return 0, movementsensor.ErrMethodUnimplementedCompassHeading
}

func (mpu *mpu6050) Position(ctx context.Context, extra map[string]interface{}) (*geo.Point, float64, error) {
	return geo.NewPoint(0, 0), 0, movementsensor.ErrMethodUnimplementedPosition
}

func (mpu *mpu6050) Accuracy(ctx context.Context, extra map[string]interface{}) (map[string]float32, error) {
	return map[string]float32{}, movementsensor.ErrMethodUnimplementedAccuracy
}

func (mpu *mpu6050) Readings(ctx context.Context, extra map[string]interface{}) (map[string]interface{}, error) {
	mpu.mu.Lock()
	defer mpu.mu.Unlock()

	readings := make(map[string]interface{})
	readings["linear_acceleration"] = mpu.linearAcceleration
	readings["temperature_celsius"] = mpu.temperature
	readings["angular_velocity"] = mpu.angularVelocity

	return readings, mpu.err.Get()
}

func (mpu *mpu6050) Properties(ctx context.Context, extra map[string]interface{}) (*movementsensor.Properties, error) {
	return &movementsensor.Properties{
		AngularVelocitySupported:    true,
		LinearAccelerationSupported: true,
	}, nil
}

func (mpu *mpu6050) Close(ctx context.Context) error {
	mpu.cancelFunc()
	mpu.activeBackgroundWorkers.Wait()

	mpu.mu.Lock()
	defer mpu.mu.Unlock()
	// Set the Sleep bit (bit 6) in the power control register (register 107).
	err := mpu.writeByte(ctx, 107, 1<<6)
	if err != nil {
		mpu.logger.Error(err)
	}
	return err
}
