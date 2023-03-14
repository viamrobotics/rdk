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
