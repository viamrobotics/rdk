package ams

import (
	"context"
	"math"
	"testing"

	"github.com/edaniels/golog"
	"go.viam.com/test"

	"go.viam.com/rdk/components/board"
	"go.viam.com/rdk/components/encoder"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/testutils/inject"
)

var model = resource.NewDefaultModel("AMS-AS5048")

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

func setupDependencies(mockData []byte) (config.Component, registry.Dependencies) {
	testBoardName := "board"
	i2cName := "i2c"

	i2cConf := &I2CAttrConfig{
		I2CBus:  i2cName,
		I2CAddr: 64,
	}

	cfg := config.Component{
		Name:  "encoder",
		Model: model,
		Type:  encoder.SubtypeName,
		ConvertedAttributes: &AttrConfig{
			BoardName:      testBoardName,
			ConnectionType: "i2c",
			I2CAttrConfig:  i2cConf,
		},
	}

	i2cHandle := &inject.I2CHandle{}
	i2cHandle.ReadByteDataFunc = func(ctx context.Context, register byte) (byte, error) {
		return mockData[register], nil
	}
	i2cHandle.WriteByteDataFunc = func(ctx context.Context, b1, b2 byte) error {
		mockData[b1] = b2
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
	return cfg, registry.Dependencies{
		resource.NameFromSubtype(board.Subtype, testBoardName): mockBoard,
	}
}

func TestAMSEncoder(t *testing.T) {
	ctx := context.Background()

	positionMockData := make([]byte, 256)
	positionMockData[0xFE] = 100
	positionMockData[0xFF] = 60

	logger := golog.NewTestLogger(t)
	cfg, deps := setupDependencies(positionMockData)
	enc, err := newAS5048Encoder(ctx, deps, cfg, logger)
	test.That(t, err, test.ShouldBeNil)

	t.Run("test automatically set to type ticks", func(t *testing.T) {
		enc.(*Encoder).position = 142
		pos, posType, _ := enc.GetPosition(ctx, nil, nil)
		test.That(t, pos, test.ShouldAlmostEqual, 0.4, 0.1)
		test.That(t, posType, test.ShouldEqual, 1)
	})
	t.Run("test ticks type from input", func(t *testing.T) {
		pos, posType, _ := enc.GetPosition(ctx, encoder.PositionTypeTICKS.Enum(), nil)
		test.That(t, pos, test.ShouldAlmostEqual, 0.4, 0.1)
		test.That(t, posType, test.ShouldEqual, 1)
	})
	t.Run("test degrees type from input", func(t *testing.T) {
		pos, posType, _ := enc.GetPosition(ctx, encoder.PositionTypeDEGREES.Enum(), nil)
		test.That(t, pos, test.ShouldAlmostEqual, 142, 0.1)
		test.That(t, posType, test.ShouldEqual, 2)
	})
	t.Run("test reset", func(t *testing.T) {
		enc.ResetPosition(ctx, nil)

		pos, posType, _ := enc.GetPosition(ctx, encoder.PositionTypeDEGREES.Enum(), nil)
		test.That(t, pos, test.ShouldAlmostEqual, 0, 0.1)
		test.That(t, posType, test.ShouldEqual, 2)

		pos, posType, _ = enc.GetPosition(ctx, encoder.PositionTypeTICKS.Enum(), nil)
		test.That(t, pos, test.ShouldAlmostEqual, 0, 0.1)
		test.That(t, posType, test.ShouldEqual, 1)
	})
}
