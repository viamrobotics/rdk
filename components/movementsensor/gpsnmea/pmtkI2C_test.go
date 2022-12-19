package gpsnmea

import (
	"context"
	"testing"

	"github.com/edaniels/golog"
	"go.viam.com/test"
	gutils "go.viam.com/utils"

	"go.viam.com/rdk/components/board"
	"go.viam.com/rdk/components/movementsensor"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/utils"
)

const (
	testBoardName = "board1"
	testBusName   = "i2c1"
)

func setupDependencies(t *testing.T) registry.Dependencies {
	t.Helper()

	deps := make(registry.Dependencies)

	actualBoard := newBoard(testBoardName)
	deps[board.Named(testBoardName)] = actualBoard

	return deps
}

func TestValidateI2C(t *testing.T) {
	fakecfg := &I2CAttrConfig{I2CBus: "some-bus"}

	path := "path"
	err := fakecfg.ValidateI2C(path)
	test.That(t, err, test.ShouldBeError,
		gutils.NewConfigValidationFieldRequiredError(path, "i2c_addr"))

	fakecfg.I2cAddr = 66
	err = fakecfg.ValidateI2C(path)
	test.That(t, err, test.ShouldBeNil)
}

func TestNewI2CMovementSensor(t *testing.T) {
	deps := setupDependencies(t)

	cfig := config.Component{
		Name:  "movementsensor1",
		Model: resource.NewDefaultModel("gps-nmea"),
		Type:  movementsensor.SubtypeName,
	}

	logger := golog.NewTestLogger(t)
	ctx := context.Background()

	g, err := newNMEAGPS(ctx, deps, cfig, logger)
	test.That(t, g, test.ShouldBeNil)
	test.That(t, err, test.ShouldBeError,
		utils.NewUnexpectedTypeError(&AttrConfig{},
			cfig.ConvertedAttributes))

	cfig = config.Component{
		Name:  "movementsensor2",
		Model: resource.NewDefaultModel("gps-nmea"),
		Type:  movementsensor.SubtypeName,
		ConvertedAttributes: &AttrConfig{
			ConnectionType: "I2C",
			Board:          testBoardName,
			DisableNMEA:    false,
			I2CAttrConfig:  &I2CAttrConfig{I2CBus: testBusName},
		},
	}
	g, err = newNMEAGPS(ctx, deps, cfig, logger)
	passErr := "board " + testBoardName + " is not local"
	if err == nil || err.Error() != passErr {
		test.That(t, err, test.ShouldBeNil)
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
	g.data = gpsData{
		location:   loc,
		alt:        alt,
		speed:      speed,
		vDOP:       vAcc,
		hDOP:       hAcc,
		satsInView: totalSats,
		satsInUse:  activeSats,
		valid:      valid,
		fixQuality: fix,
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

	err := g.Close()
	test.That(t, err, test.ShouldBeNil)
}

func newBoard(name string) *mock {
	return &mock{
		Name: name,
		i2cs: []string{"i2c1"},
		i2c:  &mockI2C{1},
	}
}

// Mock I2C.
type mock struct {
	board.LocalBoard
	Name string

	i2cs []string
	i2c  *mockI2C
}

type mockI2C struct{ handleCount int }

func (m *mock) I2CNames() []string {
	return m.i2cs
}

func (m *mock) I2CByName(name string) (*mockI2C, bool) {
	if len(m.i2cs) == 0 {
		return nil, false
	}
	return m.i2c, true
}
