//go:build linux

package ams

import (
	"context"
	"math"
	"testing"

	"go.viam.com/test"
	"go.viam.com/utils/testutils"

	"go.viam.com/rdk/components/board/genericlinux/buses"
	"go.viam.com/rdk/components/encoder"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/testutils/inject"
)

func TestConvertBytesToAngle(t *testing.T) {
	// 180 degrees
	msB := byte(math.Pow(2.0, 7.0))
	lsB := byte(0)
	deg := convertBytesToAngle(msB, lsB)
	test.That(t, deg, test.ShouldEqual, 180.0)

	// 270 degrees
	msB = byte(math.Pow(2.0, 6.0) + math.Pow(2.0, 7.0))
	lsB = byte(0)
	deg = convertBytesToAngle(msB, lsB)
	test.That(t, deg, test.ShouldEqual, 270.0)

	// 219.990234 degrees
	// 10011100011100 in binary, msB = 10011100, lsB = 00011100
	msB = byte(156)
	lsB = byte(28)
	deg = convertBytesToAngle(msB, lsB)
	test.That(t, deg, test.ShouldAlmostEqual, 219.990234, 1e-6)
}

func setupDependencies(mockData []byte) (resource.Config, resource.Dependencies, buses.I2C) {
	i2cConf := &I2CConfig{
		I2CBus:  "1",
		I2CAddr: 64,
	}

	cfg := resource.Config{
		Name:  "encoder",
		Model: model,
		API:   encoder.API,
		ConvertedAttributes: &Config{
			ConnectionType: "i2c",
			I2CConfig:      i2cConf,
		},
	}

	i2cHandle := &inject.I2CHandle{}
	i2cHandle.ReadByteDataFunc = func(ctx context.Context, register byte) (byte, error) {
		return mockData[register], nil
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

func TestAMSEncoder(t *testing.T) {
	ctx := context.Background()

	positionMockData := make([]byte, 256)
	positionMockData[0xFE] = 100
	positionMockData[0xFF] = 60

	logger := logging.NewTestLogger(t)
	cfg, deps, bus := setupDependencies(positionMockData)
	enc, err := makeAS5048Encoder(ctx, deps, cfg, logger, bus)
	test.That(t, err, test.ShouldBeNil)
	defer enc.Close(ctx)

	t.Run("test automatically set to type ticks", func(t *testing.T) {
		testutils.WaitForAssertion(t, func(tb testing.TB) {
			pos, _, _ := enc.Position(ctx, encoder.PositionTypeUnspecified, nil)
			test.That(tb, pos, test.ShouldNotEqual, 0.0)
		})
		pos, posType, _ := enc.Position(ctx, encoder.PositionTypeUnspecified, nil)
		test.That(t, pos, test.ShouldAlmostEqual, 0.4, 0.1)
		test.That(t, posType, test.ShouldEqual, 1)
	})
	t.Run("test ticks type from input", func(t *testing.T) {
		testutils.WaitForAssertion(t, func(tb testing.TB) {
			pos, _, _ := enc.Position(ctx, encoder.PositionTypeTicks, nil)
			test.That(tb, pos, test.ShouldNotEqual, 0.0)
		})
		pos, posType, _ := enc.Position(ctx, encoder.PositionTypeUnspecified, nil)
		test.That(t, pos, test.ShouldAlmostEqual, 0.4, 0.1)
		test.That(t, posType, test.ShouldEqual, 1)
	})
	t.Run("test degrees type from input", func(t *testing.T) {
		testutils.WaitForAssertion(t, func(tb testing.TB) {
			pos, _, _ := enc.Position(ctx, encoder.PositionTypeTicks, nil)
			test.That(tb, pos, test.ShouldNotEqual, 0.0)
		})
		pos, posType, _ := enc.Position(ctx, encoder.PositionTypeDegrees, nil)
		test.That(t, pos, test.ShouldAlmostEqual, 142, 0.1)
		test.That(t, posType, test.ShouldEqual, 2)
	})
}

func setupDependenciesWithWrite(mockData []byte, writeData map[byte]byte) (resource.Config, resource.Dependencies, buses.I2C) {
	i2cConf := &I2CConfig{
		I2CBus:  "1",
		I2CAddr: 64,
	}

	cfg := resource.Config{
		Name:  "encoder",
		Model: model,
		API:   encoder.API,
		ConvertedAttributes: &Config{
			ConnectionType: "i2c",
			I2CConfig:      i2cConf,
		},
	}

	i2cHandle := &inject.I2CHandle{}
	i2cHandle.ReadByteDataFunc = func(ctx context.Context, register byte) (byte, error) {
		return mockData[register], nil
	}
	i2cHandle.WriteByteDataFunc = func(ctx context.Context, b1, b2 byte) error {
		writeData[b1] = b2
		return nil
	}
	i2cHandle.CloseFunc = func() error { return nil }

	i2c := &inject.I2C{}
	i2c.OpenHandleFunc = func(addr byte) (buses.I2CHandle, error) {
		return i2cHandle, nil
	}
	return cfg, resource.Dependencies{}, i2c
}

func TestAMSEncoderReset(t *testing.T) {
	ctx := context.Background()

	positionMockData := make([]byte, 256)
	positionMockData[0xFE] = 100
	positionMockData[0xFF] = 60

	writeData := make(map[byte]byte)

	logger := logging.NewTestLogger(t)
	cfg, deps, bus := setupDependenciesWithWrite(positionMockData, writeData)
	enc, err := makeAS5048Encoder(ctx, deps, cfg, logger, bus)
	test.That(t, err, test.ShouldBeNil)
	defer enc.Close(ctx)

	t.Run("test reset", func(t *testing.T) {
		testutils.WaitForAssertion(t, func(tb testing.TB) {
			enc.ResetPosition(ctx, nil)
			pos, posType, _ := enc.Position(ctx, encoder.PositionTypeUnspecified, nil)
			test.That(tb, pos, test.ShouldAlmostEqual, 0, 0.1)
			test.That(tb, posType, test.ShouldEqual, 1)
		})

		test.That(t, writeData[0x16], test.ShouldEqual, byte(100))
		test.That(t, writeData[0x17], test.ShouldEqual, byte(60))
	})
}
