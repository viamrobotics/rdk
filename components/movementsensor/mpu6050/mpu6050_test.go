package mpu6050

import (
	"context"
	"testing"

	"github.com/edaniels/golog"
	"github.com/pkg/errors"
	"go.viam.com/test"
	"go.viam.com/utils"

	"go.viam.com/rdk/components/board"
	"go.viam.com/rdk/components/movementsensor"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/testutils/inject"
	"go.viam.com/utils/testutils"
)

func TestValidateConfig(t *testing.T) {
	boardName := "local"
	t.Run("fails with no board supplied", func(t *testing.T) {
		cfg := AttrConfig{
			I2cBus: "thing",
		}
		deps, err := cfg.Validate("path")
		expectedErr := utils.NewConfigValidationFieldRequiredError("path", "board")
		test.That(t, err, test.ShouldBeError, expectedErr)
		test.That(t, deps, test.ShouldBeEmpty)
	})

	t.Run("fails with no I2C bus", func(t *testing.T) {
		cfg := AttrConfig{
			BoardName: boardName,
		}
		deps, err := cfg.Validate("path")
		expectedErr := utils.NewConfigValidationFieldRequiredError("path", "i2c_bus")
		test.That(t, err, test.ShouldBeError, expectedErr)
		test.That(t, deps, test.ShouldBeEmpty)
	})

	t.Run("adds board name to dependencies on success", func(t *testing.T) {
		cfg := AttrConfig{
			BoardName: boardName,
			I2cBus:    "thing2",
		}
		deps, err := cfg.Validate("path")

		test.That(t, err, test.ShouldBeNil)
		test.That(t, len(deps), test.ShouldEqual, 1)
		test.That(t, deps[0], test.ShouldResemble, boardName)
	})
}

func TestInitializationFailureOnChipCommunication(t *testing.T) {
	logger := golog.NewTestLogger(t)
	testBoardName := "board"
	i2cName := "i2c"

	t.Run("fails on read error", func(t *testing.T) {
		cfg := config.Component{
			Name:  "movementsensor",
			Model: model,
			Type:  movementsensor.SubtypeName,
			ConvertedAttributes: &AttrConfig{
				BoardName: testBoardName,
				I2cBus:    i2cName,
			},
		}
		i2cHandle := &inject.I2CHandle{}
		readErr := errors.New("read error")
		i2cHandle.ReadBlockDataFunc = func(ctx context.Context, register byte, numBytes uint8) ([]byte, error) {
			if register == defaultAddressRegister {
				return nil, readErr
			}
			return []byte{}, nil
		}
		i2cHandle.CloseFunc = func() error { return nil }
		mockBoard := &inject.Board{}
		mockBoard.I2CByNameFunc = func(name string) (board.I2C, bool) {
			i2c := &inject.I2C{}
			i2c.OpenHandleFunc = func(addr byte) (board.I2CHandle, error) {
				return i2cHandle, nil
			}
			return i2c, true
		}
		deps := registry.Dependencies{
			resource.NameFromSubtype(board.Subtype, testBoardName): mockBoard,
		}
		sensor, err := NewMpu6050(context.Background(), deps, cfg, logger)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err, test.ShouldBeError, addressReadError(readErr, expectedDefaultAddress, i2cName, testBoardName))
		test.That(t, sensor, test.ShouldBeNil)
	})

	t.Run("fails on unexpected address", func(t *testing.T) {
		cfg := config.Component{
			Name:  "movementsensor",
			Model: model,
			Type:  movementsensor.SubtypeName,
			ConvertedAttributes: &AttrConfig{
				BoardName:              testBoardName,
				I2cBus:                 i2cName,
				UseAlternateI2CAddress: true,
			},
		}
		i2cHandle := &inject.I2CHandle{}
		i2cHandle.ReadBlockDataFunc = func(ctx context.Context, register byte, numBytes uint8) ([]byte, error) {
			if register == defaultAddressRegister {
				return []byte{0x64}, nil
			}
			return nil, errors.New("unexpected register")
		}
		i2cHandle.CloseFunc = func() error { return nil }
		mockBoard := &inject.Board{}
		mockBoard.I2CByNameFunc = func(name string) (board.I2C, bool) {
			i2c := &inject.I2C{}
			i2c.OpenHandleFunc = func(addr byte) (board.I2CHandle, error) {
				return i2cHandle, nil
			}
			return i2c, true
		}
		deps := registry.Dependencies{
			resource.NameFromSubtype(board.Subtype, testBoardName): mockBoard,
		}
		sensor, err := NewMpu6050(context.Background(), deps, cfg, logger)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err, test.ShouldBeError, unexpectedDeviceError(alternateAddress, 0x64))
		test.That(t, sensor, test.ShouldBeNil)
	})
}

func TestSuccessfulInitializationAndClose(t *testing.T) {
	logger := golog.NewTestLogger(t)
	testBoardName := "board"
	i2cName := "i2c"

	cfg := config.Component{
		Name:  "movementsensor",
		Model: model,
		Type:  movementsensor.SubtypeName,
		ConvertedAttributes: &AttrConfig{
			BoardName:              testBoardName,
			I2cBus:                 i2cName,
			UseAlternateI2CAddress: true,
		},
	}
	i2cHandle := &inject.I2CHandle{}
	i2cHandle.ReadBlockDataFunc = func(ctx context.Context, register byte, numBytes uint8) ([]byte, error) {
		return []byte{expectedDefaultAddress}, nil
	}
	// the only write operations that the sensor implementation performs is
	// the command to put it into either measurement mode or sleep mode,
	// and measurement mode results from a write of 0, so if is closeWasCalled is toggled
	// we know Close() was successfully called
	closeWasCalled := false
	i2cHandle.WriteByteDataFunc = func(ctx context.Context, register, data byte) error {
		if data == 1<<6 {
			closeWasCalled = true
		}
		return nil
	}
	i2cHandle.CloseFunc = func() error { return nil }
	mockBoard := &inject.Board{}
	mockBoard.I2CByNameFunc = func(name string) (board.I2C, bool) {
		i2c := &inject.I2C{}
		i2c.OpenHandleFunc = func(addr byte) (board.I2CHandle, error) {
			return i2cHandle, nil
		}
		return i2c, true
	}
	deps := registry.Dependencies{
		resource.NameFromSubtype(board.Subtype, testBoardName): mockBoard,
	}
	sensor, err := NewMpu6050(context.Background(), deps, cfg, logger)
	test.That(t, err, test.ShouldBeNil)
	err = utils.TryClose(context.Background(), sensor)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, closeWasCalled, test.ShouldBeTrue)
}

func TestLinearAcceleration(t *testing.T) {
	bytesToReturn := make([]byte, 16)
	// x-accel
	bytesToReturn[0] = 64
	bytesToReturn[1] = 0
	expectedX := 9810.0
	// y-accel
	bytesToReturn[2] = 32
	bytesToReturn[3] = 0
	expectedY := 4905.0
	// z-accel
	bytesToReturn[4] = 16
	bytesToReturn[5] = 0
	expectedZ := 2452.5

	logger := golog.NewTestLogger(t)
	testBoardName := "board"
	i2cName := "i2c"

	cfg := config.Component{
		Name:  "movementsensor",
		Model: model,
		Type:  movementsensor.SubtypeName,
		ConvertedAttributes: &AttrConfig{
			BoardName:              testBoardName,
			I2cBus:                 i2cName,
			UseAlternateI2CAddress: true,
		},
	}
	i2cHandle := &inject.I2CHandle{}
	i2cHandle.ReadBlockDataFunc = func(ctx context.Context, register byte, numBytes uint8) ([]byte, error) {
		if register == defaultAddressRegister {
			return []byte{expectedDefaultAddress}, nil
		}
		return bytesToReturn, nil
	}
	i2cHandle.WriteByteDataFunc = func(ctx context.Context, b1, b2 byte) error {
		return nil
	}
	i2cHandle.CloseFunc = func() error { return nil }
	mockBoard := &inject.Board{}
	mockBoard.I2CByNameFunc = func(name string) (board.I2C, bool) {
		i2c := &inject.I2C{}
		i2c.OpenHandleFunc = func(addr byte) (board.I2CHandle, error) {
			return i2cHandle, nil
		}
		return i2c, true
	}
	deps := registry.Dependencies{
		resource.NameFromSubtype(board.Subtype, testBoardName): mockBoard,
	}
	sensor, err := NewMpu6050(context.Background(), deps, cfg, logger)
	test.That(t, err, test.ShouldBeNil)
	defer utils.TryClose(context.Background(), sensor)
	mpuStruct, isMpuStruct := sensor.(*mpu6050)
	test.That(t, isMpuStruct, test.ShouldBeTrue)
	testutils.WaitForAssertion(t, func(tb testing.TB) {
		test.That(tb, mpuStruct.linearAcceleration, test.ShouldNotBeZeroValue)
	})
	accel, err := sensor.LinearAcceleration(context.Background(), nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, accel.X, test.ShouldEqual, expectedX)
	test.That(t, accel.Y, test.ShouldEqual, expectedY)
	test.That(t, accel.Z, test.ShouldEqual, expectedZ)
}

func TestAngularVelocity(t *testing.T) {
	bytesToReturn := make([]byte, 16)
	// x-vel
	bytesToReturn[8] = 64
	bytesToReturn[9] = 0
	expectedX := 125.0
	// y-accel
	bytesToReturn[10] = 32
	bytesToReturn[11] = 0
	expectedY := 62.5
	// z-accel
	bytesToReturn[12] = 16
	bytesToReturn[13] = 0
	expectedZ := 31.25

	logger := golog.NewTestLogger(t)
	testBoardName := "board"
	i2cName := "i2c"

	cfg := config.Component{
		Name:  "movementsensor",
		Model: model,
		Type:  movementsensor.SubtypeName,
		ConvertedAttributes: &AttrConfig{
			BoardName:              testBoardName,
			I2cBus:                 i2cName,
			UseAlternateI2CAddress: true,
		},
	}
	i2cHandle := &inject.I2CHandle{}
	i2cHandle.ReadBlockDataFunc = func(ctx context.Context, register byte, numBytes uint8) ([]byte, error) {
		if register == defaultAddressRegister {
			return []byte{expectedDefaultAddress}, nil
		}
		return bytesToReturn, nil
	}
	i2cHandle.WriteByteDataFunc = func(ctx context.Context, b1, b2 byte) error {
		return nil
	}
	i2cHandle.CloseFunc = func() error { return nil }
	mockBoard := &inject.Board{}
	mockBoard.I2CByNameFunc = func(name string) (board.I2C, bool) {
		i2c := &inject.I2C{}
		i2c.OpenHandleFunc = func(addr byte) (board.I2CHandle, error) {
			return i2cHandle, nil
		}
		return i2c, true
	}
	deps := registry.Dependencies{
		resource.NameFromSubtype(board.Subtype, testBoardName): mockBoard,
	}
	sensor, err := NewMpu6050(context.Background(), deps, cfg, logger)
	test.That(t, err, test.ShouldBeNil)
	defer utils.TryClose(context.Background(), sensor)
	mpuStruct, isMpuStruct := sensor.(*mpu6050)
	test.That(t, isMpuStruct, test.ShouldBeTrue)
	testutils.WaitForAssertion(t, func(tb testing.TB) {
		test.That(tb, mpuStruct.angularVelocity, test.ShouldNotBeZeroValue)
	})
	angVel, err := sensor.AngularVelocity(context.Background(), nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, angVel.X, test.ShouldEqual, expectedX)
	test.That(t, angVel.Y, test.ShouldEqual, expectedY)
	test.That(t, angVel.Z, test.ShouldEqual, expectedZ)
}

func TestTemperature(t *testing.T) {
	bytesToReturn := make([]byte, 16)
	bytesToReturn[6] = 231
	bytesToReturn[7] = 202
	expectedTemp := 18.3

	logger := golog.NewTestLogger(t)
	testBoardName := "board"
	i2cName := "i2c"

	cfg := config.Component{
		Name:  "movementsensor",
		Model: model,
		Type:  movementsensor.SubtypeName,
		ConvertedAttributes: &AttrConfig{
			BoardName:              testBoardName,
			I2cBus:                 i2cName,
			UseAlternateI2CAddress: true,
		},
	}
	i2cHandle := &inject.I2CHandle{}
	i2cHandle.ReadBlockDataFunc = func(ctx context.Context, register byte, numBytes uint8) ([]byte, error) {
		if register == defaultAddressRegister {
			return []byte{expectedDefaultAddress}, nil
		}
		return bytesToReturn, nil
	}
	i2cHandle.WriteByteDataFunc = func(ctx context.Context, b1, b2 byte) error {
		return nil
	}
	i2cHandle.CloseFunc = func() error { return nil }
	mockBoard := &inject.Board{}
	mockBoard.I2CByNameFunc = func(name string) (board.I2C, bool) {
		i2c := &inject.I2C{}
		i2c.OpenHandleFunc = func(addr byte) (board.I2CHandle, error) {
			return i2cHandle, nil
		}
		return i2c, true
	}
	deps := registry.Dependencies{
		resource.NameFromSubtype(board.Subtype, testBoardName): mockBoard,
	}
	sensor, err := NewMpu6050(context.Background(), deps, cfg, logger)
	test.That(t, err, test.ShouldBeNil)
	defer utils.TryClose(context.Background(), sensor)
	mpuStruct, isMpuStruct := sensor.(*mpu6050)
	test.That(t, isMpuStruct, test.ShouldBeTrue)
	testutils.WaitForAssertion(t, func(tb testing.TB) {
		test.That(tb, mpuStruct.temperature, test.ShouldNotBeZeroValue)
	})
	test.That(t, mpuStruct.temperature, test.ShouldAlmostEqual, expectedTemp, 0.001)
}
