package gpsnmea

import (
	"context"
	"testing"

	"github.com/edaniels/golog"
	"go.viam.com/test"
	gutils "go.viam.com/utils"

	"go.viam.com/rdk/components/board"
	"go.viam.com/rdk/components/movementsensor"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/testutils/inject"
	"go.viam.com/rdk/utils"
)

const (
	testBoardName = "board1"
	testBusName   = "i2c1"
)

func setupDependencies(t *testing.T) resource.Dependencies {
	t.Helper()

	deps := make(resource.Dependencies)

	actualBoard := inject.NewBoard(testBoardName)
	i2c1 := &inject.I2C{}
	handle1 := &inject.I2CHandle{}
	handle1.WriteFunc = func(ctx context.Context, b []byte) error {
		return nil
	}
	handle1.ReadFunc = func(ctx context.Context, count int) ([]byte, error) {
		return nil, nil
	}
	handle1.CloseFunc = func() error {
		return nil
	}
	i2c1.OpenHandleFunc = func(addr byte) (board.I2CHandle, error) {
		return handle1, nil
	}
	actualBoard.I2CByNameFunc = func(name string) (board.I2C, bool) {
		return i2c1, true
	}

	deps[board.Named(testBoardName)] = actualBoard

	return deps
}

func TestValidateI2C(t *testing.T) {
	fakecfg := &I2CConfig{Board: testBoardName, I2CBus: "some-bus"}

	path := "path"
	err := fakecfg.validateI2C(path)
	test.That(t, err, test.ShouldBeError,
		gutils.NewConfigValidationFieldRequiredError(path, "i2c_addr"))

	fakecfg.I2CAddr = 66
	err = fakecfg.validateI2C(path)
	test.That(t, err, test.ShouldBeNil)
}

func TestNewI2CMovementSensor(t *testing.T) {
	deps := setupDependencies(t)

	conf := resource.Config{
		Name:  "movementsensor1",
		Model: resource.DefaultModelFamily.WithModel("gps-nmea"),
		API:   movementsensor.API,
	}

	logger := golog.NewTestLogger(t)
	ctx := context.Background()

	g, err := newNMEAGPS(ctx, deps, conf, logger)
	test.That(t, g, test.ShouldBeNil)
	test.That(t, err, test.ShouldBeError,
		utils.NewUnexpectedTypeError[*Config](conf.ConvertedAttributes))

	conf = resource.Config{
		Name:  "movementsensor2",
		Model: resource.DefaultModelFamily.WithModel("gps-nmea"),
		API:   movementsensor.API,
		ConvertedAttributes: &Config{
			ConnectionType: "I2C",
			DisableNMEA:    false,
			I2CConfig:      &I2CConfig{I2CBus: testBusName, Board: testBoardName},
		},
	}
	g, err = newNMEAGPS(ctx, deps, conf, logger)
	passErr := "board " + testBoardName + " is not local"
	if err == nil || err.Error() != passErr {
		test.That(t, err, test.ShouldBeNil)
		test.That(t, g.Close(context.Background()), test.ShouldBeNil)
		test.That(t, g, test.ShouldNotBeNil)
	}
}

func TestReadingsI2C(t *testing.T) {
	logger := golog.NewTestLogger(t)
	ctx := context.Background()
	cancelCtx, cancelFunc := context.WithCancel(ctx)
	g := &PmtkI2CNMEAMovementSensor{
		cancelCtx:  cancelCtx,
		cancelFunc: cancelFunc,
		logger:     logger,
	}
	g.data = GPSData{
		Location:   loc,
		Alt:        alt,
		Speed:      speed,
		VDOP:       vAcc,
		HDOP:       hAcc,
		SatsInView: totalSats,
		SatsInUse:  activeSats,
		valid:      valid,
		FixQuality: fix,
	}

	g.bus = nil
	g.addr = 66

	bus, addr := g.GetBusAddr()
	test.That(t, bus, test.ShouldBeNil)
	test.That(t, addr, test.ShouldEqual, 66)

	loc1, alt1, err := g.Position(ctx, make(map[string]interface{}))
	test.That(t, err, test.ShouldBeNil)
	test.That(t, loc1, test.ShouldEqual, loc)
	test.That(t, alt1, test.ShouldEqual, alt)

	speed1, err := g.LinearVelocity(ctx, make(map[string]interface{}))
	test.That(t, err, test.ShouldBeNil)
	test.That(t, speed1.Y, test.ShouldEqual, speed)

	fix1, err := g.ReadFix(ctx)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, fix1, test.ShouldEqual, fix)
}

func TestCloseI2C(t *testing.T) {
	logger := golog.NewTestLogger(t)
	ctx := context.Background()
	cancelCtx, cancelFunc := context.WithCancel(ctx)
	g := &PmtkI2CNMEAMovementSensor{
		cancelCtx:  cancelCtx,
		cancelFunc: cancelFunc,
		logger:     logger,
	}

	err := g.Close(context.Background())
	test.That(t, err, test.ShouldBeNil)
}
