//go:build linux

package adxl345

import (
	"context"
	"testing"
	"time"

	"github.com/pkg/errors"
	"go.viam.com/test"
	"go.viam.com/utils"
	"go.viam.com/utils/testutils"

	"go.viam.com/rdk/components/board"
	"go.viam.com/rdk/components/board/genericlinux/buses"
	"go.viam.com/rdk/components/movementsensor"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/testutils/inject"
)

func nowNanosTest() uint64 {
	return uint64(time.Now().UnixNano())
}

func setupDependencies(mockData []byte) (resource.Config, resource.Dependencies, buses.I2C) {
	cfg := resource.Config{
		Name:  "movementsensor",
		Model: model,
		API:   movementsensor.API,
		ConvertedAttributes: &Config{
			I2cBus:                 "2",
			UseAlternateI2CAddress: true,
		},
	}

	i2cHandle := &inject.I2CHandle{}
	i2cHandle.ReadBlockDataFunc = func(ctx context.Context, register byte, numBytes uint8) ([]byte, error) {
		if register == 0 {
			return []byte{0xE5}, nil
		}
		return mockData, nil
	}
	i2cHandle.WriteByteDataFunc = func(ctx context.Context, b1, b2 byte) error {
		return nil
	}
	i2cHandle.CloseFunc = func() error { return nil }

	i2c := &inject.I2C{}
	i2c.OpenHandleFunc = func(addr byte) (buses.I2CHandle, error) {
		return i2cHandle, nil
	}

	return cfg, resource.Dependencies{}, i2c
}

func sendInterrupt(ctx context.Context, adxl movementsensor.MovementSensor, t *testing.T, interrupt board.DigitalInterrupt, key string) {
	interrupt.Tick(ctx, true, nowNanosTest())
	testutils.WaitForAssertion(t, func(tb testing.TB) {
		readings, err := adxl.Readings(ctx, map[string]interface{}{})
		test.That(tb, err, test.ShouldBeNil)
		test.That(tb, readings[key], test.ShouldNotBeZeroValue)
	})
}

func TestValidateConfig(t *testing.T) {
	boardName := "local"
	t.Run("fails when interrupts are used without a supplied board", func(t *testing.T) {
		tapCfg := TapConfig{
			AccelerometerPin: 1,
			InterruptPin:     "on_missing_board",
		}
		cfg := Config{
			I2cBus:    "3",
			SingleTap: &tapCfg,
		}
		deps, err := cfg.Validate("path")
		expectedErr := utils.NewConfigValidationFieldRequiredError("path", "board")
		test.That(t, err, test.ShouldBeError, expectedErr)
		test.That(t, deps, test.ShouldBeEmpty)
	})

	t.Run("fails with no I2C bus", func(t *testing.T) {
		cfg := Config{}
		deps, err := cfg.Validate("path")
		expectedErr := utils.NewConfigValidationFieldRequiredError("path", "i2c_bus")
		test.That(t, err, test.ShouldBeError, expectedErr)
		test.That(t, deps, test.ShouldBeEmpty)
	})

	t.Run("passes with no board supplied, no dependencies", func(t *testing.T) {
		cfg := Config{
			I2cBus: "3",
		}
		deps, err := cfg.Validate("path")
		test.That(t, err, test.ShouldBeNil)
		test.That(t, len(deps), test.ShouldEqual, 0)
	})

	t.Run("adds board name to dependencies on success with interrupts", func(t *testing.T) {
		tapCfg := TapConfig{
			AccelerometerPin: 1,
			InterruptPin:     "on_present_board",
		}
		cfg := Config{
			BoardName: boardName,
			I2cBus:    "2",
			SingleTap: &tapCfg,
		}
		deps, err := cfg.Validate("path")

		test.That(t, err, test.ShouldBeNil)
		test.That(t, len(deps), test.ShouldEqual, 1)
		test.That(t, deps[0], test.ShouldResemble, boardName)
	})
}

func TestInitializationFailureOnChipCommunication(t *testing.T) {
	logger := logging.NewTestLogger(t)

	t.Run("fails on read error", func(t *testing.T) {
		cfg := resource.Config{
			Name:  "movementsensor",
			Model: model,
			API:   movementsensor.API,
			ConvertedAttributes: &Config{
				I2cBus: "2",
			},
		}

		i2cHandle := &inject.I2CHandle{}
		readErr := errors.New("read error")
		i2cHandle.ReadBlockDataFunc = func(ctx context.Context, register byte, numBytes uint8) ([]byte, error) {
			if register == deviceIDRegister {
				return nil, readErr
			}
			return []byte{}, nil
		}
		i2cHandle.CloseFunc = func() error { return nil }

		i2c := &inject.I2C{}
		i2c.OpenHandleFunc = func(addr byte) (buses.I2CHandle, error) {
			return i2cHandle, nil
		}

		sensor, err := makeAdxl345(context.Background(), resource.Dependencies{}, cfg, logger, i2c)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, sensor, test.ShouldBeNil)
	})
}

func TestInterrupts(t *testing.T) {
	ctx := context.Background()

	interrupt := &board.BasicDigitalInterrupt{}

	mockBoard := &inject.Board{}
	mockBoard.DigitalInterruptByNameFunc = func(name string) (board.DigitalInterrupt, bool) { return interrupt, true }

	i2cHandle := &inject.I2CHandle{}
	i2cHandle.CloseFunc = func() error { return nil }
	i2cHandle.WriteByteDataFunc = func(context.Context, byte, byte) error { return nil }
	// The data returned from the readByteFunction is intended to signify which interrupts have gone off
	i2cHandle.ReadByteDataFunc = func(context.Context, byte) (byte, error) { return byte(1<<6 + 1<<2), nil }
	// i2cHandle.ReadBlockDataFunc gets called multiple times. The first time we need the first byte to be 0xE5 and the next
	// time we need 6 bytes. This return provides more data than necessary for the first call to the function but allows
	// both calls to it to work properly.
	i2cHandle.ReadBlockDataFunc = func(context.Context, byte, uint8) ([]byte, error) {
		return []byte{byte(0xE5), byte(0x1), byte(0x2), byte(0x3), byte(0x4), byte(0x5), byte(0x6)}, nil
	}

	i2c := &inject.I2C{}
	i2c.OpenHandleFunc = func(addr byte) (buses.I2CHandle, error) { return i2cHandle, nil }

	logger := logging.NewTestLogger(t)

	deps := resource.Dependencies{
		resource.NewName(board.API, "board"): mockBoard,
	}

	tap := &TapConfig{
		AccelerometerPin: 1,
		InterruptPin:     "int1",
	}

	ff := &FreeFallConfig{
		AccelerometerPin: 1,
		InterruptPin:     "int1",
	}

	cfg := resource.Config{
		Name:  "movementsensor",
		Model: model,
		API:   movementsensor.API,
		ConvertedAttributes: &Config{
			BoardName: "board",
			I2cBus:    "3",
			SingleTap: tap,
			FreeFall:  ff,
		},
	}

	t.Run("new adxl has interrupt counts set to 0", func(t *testing.T) {
		adxl, err := makeAdxl345(ctx, deps, cfg, logger, i2c)
		test.That(t, err, test.ShouldBeNil)

		readings, err := adxl.Readings(ctx, map[string]interface{}{})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, readings["freefall_count"], test.ShouldEqual, 0)
		test.That(t, readings["single_tap_count"], test.ShouldEqual, 0)
	})

	t.Run("interrupts have been found correctly when both are configured to the same pin", func(t *testing.T) {
		adxl, err := makeAdxl345(ctx, deps, cfg, logger, i2c)
		test.That(t, err, test.ShouldBeNil)

		sendInterrupt(ctx, adxl, t, interrupt, "freefall_count")

		readings, err := adxl.Readings(ctx, map[string]interface{}{})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, readings["freefall_count"], test.ShouldEqual, 1)
		test.That(t, readings["single_tap_count"], test.ShouldEqual, 1)
	})

	t.Run("interrupts have been found correctly only tap has been configured", func(t *testing.T) {
		cfg := resource.Config{
			Name:  "movementsensor",
			Model: model,
			API:   movementsensor.API,
			ConvertedAttributes: &Config{
				BoardName: "board",
				I2cBus:    "3",
				SingleTap: tap,
			},
		}

		adxl, err := makeAdxl345(ctx, deps, cfg, logger, i2c)
		test.That(t, err, test.ShouldBeNil)

		sendInterrupt(ctx, adxl, t, interrupt, "single_tap_count")

		readings, err := adxl.Readings(ctx, map[string]interface{}{})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, readings["freefall_count"], test.ShouldEqual, 0)
		test.That(t, readings["single_tap_count"], test.ShouldEqual, 1)
	})

	t.Run("interrupts have been found correctly only freefall has been configured", func(t *testing.T) {
		cfg = resource.Config{
			Name:  "movementsensor",
			Model: model,
			API:   movementsensor.API,
			ConvertedAttributes: &Config{
				BoardName: "board",
				I2cBus:    "3",
				FreeFall:  ff,
			},
		}

		adxl, err := makeAdxl345(ctx, deps, cfg, logger, i2c)
		test.That(t, err, test.ShouldBeNil)

		sendInterrupt(ctx, adxl, t, interrupt, "freefall_count")

		readings, err := adxl.Readings(ctx, map[string]interface{}{})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, readings["freefall_count"], test.ShouldEqual, 1)
		test.That(t, readings["single_tap_count"], test.ShouldEqual, 0)
	})
}

func TestReadInterrupts(t *testing.T) {
	ctx := context.Background()
	cancelContext, cancelFunc := context.WithCancel(ctx)
	i2cHandle := &inject.I2CHandle{}
	i2cHandle.CloseFunc = func() error { return nil }
	i2c := &inject.I2C{}
	i2c.OpenHandleFunc = func(addr byte) (buses.I2CHandle, error) {
		return i2cHandle, nil
	}

	t.Run("increments tap and freefall counts when both interrupts have gone off", func(t *testing.T) {
		i2cHandle.ReadBlockDataFunc = func(context.Context, byte, uint8) ([]byte, error) {
			intSourceRegister := byte(1<<6) + byte(1<<2)
			return []byte{intSourceRegister}, nil
		}

		sensor := &adxl345{
			bus:               i2c,
			interruptsFound:   map[InterruptID]int{},
			cancelContext:     cancelContext,
			cancelFunc:        cancelFunc,
			interruptsEnabled: byte(1<<6 + 1<<2),
		}
		sensor.readInterrupts(sensor.cancelContext)
		test.That(t, sensor.interruptsFound[SingleTap], test.ShouldEqual, 1)
		test.That(t, sensor.interruptsFound[FreeFall], test.ShouldEqual, 1)
	})

	t.Run("increments freefall count only when freefall has gone off", func(t *testing.T) {
		i2cHandle.ReadBlockDataFunc = func(context.Context, byte, uint8) ([]byte, error) {
			intSourceRegister := byte(1 << 2)
			return []byte{intSourceRegister}, nil
		}

		sensor := &adxl345{
			bus:               i2c,
			interruptsFound:   map[InterruptID]int{},
			cancelContext:     cancelContext,
			cancelFunc:        cancelFunc,
			interruptsEnabled: byte(1<<6 + 1<<2),
		}
		sensor.readInterrupts(sensor.cancelContext)
		test.That(t, sensor.interruptsFound[SingleTap], test.ShouldEqual, 0)
		test.That(t, sensor.interruptsFound[FreeFall], test.ShouldEqual, 1)
	})

	t.Run("increments tap count only when only tap has gone off", func(t *testing.T) {
		i2cHandle.ReadBlockDataFunc = func(context.Context, byte, uint8) ([]byte, error) {
			intSourceRegister := byte(1 << 6)
			return []byte{intSourceRegister}, nil
		}

		sensor := &adxl345{
			bus:               i2c,
			interruptsFound:   map[InterruptID]int{},
			cancelContext:     cancelContext,
			cancelFunc:        cancelFunc,
			interruptsEnabled: byte(1<<6 + 1<<2),
		}
		sensor.readInterrupts(sensor.cancelContext)
		test.That(t, sensor.interruptsFound[SingleTap], test.ShouldEqual, 1)
		test.That(t, sensor.interruptsFound[FreeFall], test.ShouldEqual, 0)
	})

	t.Run("does not increment counts when neither interrupt has gone off", func(t *testing.T) {
		i2cHandle.ReadBlockDataFunc = func(context.Context, byte, uint8) ([]byte, error) {
			intSourceRegister := byte(0)
			return []byte{intSourceRegister}, nil
		}

		sensor := &adxl345{
			bus:               i2c,
			interruptsFound:   map[InterruptID]int{},
			cancelContext:     cancelContext,
			cancelFunc:        cancelFunc,
			interruptsEnabled: byte(1<<6 + 1<<2),
		}
		sensor.readInterrupts(sensor.cancelContext)
		test.That(t, sensor.interruptsFound[SingleTap], test.ShouldEqual, 0)
		test.That(t, sensor.interruptsFound[FreeFall], test.ShouldEqual, 0)
	})
}

func TestLinearAcceleration(t *testing.T) {
	linearAccelMockData := make([]byte, 16)
	// x-accel
	linearAccelMockData[0] = 40
	linearAccelMockData[1] = 0
	expectedAccelX := 1.5328125000000001
	// y-accel
	linearAccelMockData[2] = 50
	linearAccelMockData[3] = 0
	expectedAccelY := 1.916015625
	// z-accel
	linearAccelMockData[4] = 80
	linearAccelMockData[5] = 0
	expectedAccelZ := 3.0656250000000003

	logger := logging.NewTestLogger(t)
	cfg, deps, i2c := setupDependencies(linearAccelMockData)
	sensor, err := makeAdxl345(context.Background(), deps, cfg, logger, i2c)
	test.That(t, err, test.ShouldBeNil)
	defer sensor.Close(context.Background())
	testutils.WaitForAssertion(t, func(tb testing.TB) {
		linAcc, err := sensor.LinearAcceleration(context.Background(), nil)
		test.That(tb, err, test.ShouldBeNil)
		test.That(tb, linAcc, test.ShouldNotBeZeroValue)
	})
	accel, err := sensor.LinearAcceleration(context.Background(), nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, accel.X, test.ShouldEqual, expectedAccelX)
	test.That(t, accel.Y, test.ShouldEqual, expectedAccelY)
	test.That(t, accel.Z, test.ShouldEqual, expectedAccelZ)
}
