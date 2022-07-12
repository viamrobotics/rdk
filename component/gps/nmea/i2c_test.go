package nmea

import (
	"context"
	"testing"

	"github.com/edaniels/golog"
	"go.viam.com/test"

	"go.viam.com/rdk/component/board"
	"go.viam.com/rdk/component/gps"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/registry"
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
	fakecfg := &I2CAttrConfig{}
	err := fakecfg.ValidateI2C("path")
	test.That(t, err.Error(), test.ShouldContainSubstring, "expected nonempty board")

	fakecfg.Board = "some-board"
	err = fakecfg.ValidateI2C("path")
	test.That(t, err.Error(), test.ShouldContainSubstring, "expected nonempty bus")

	fakecfg.Bus = "some-bus"
	err = fakecfg.ValidateI2C("path")
	test.That(t, err.Error(), test.ShouldContainSubstring, "expected nonempty i2c address")

	fakecfg.I2cAddr = 66
	err = fakecfg.ValidateI2C("path")
	test.That(t, err, test.ShouldBeNil)
}

func TestNewI2CGPS(t *testing.T) {
	deps := setupDependencies(t)
	
	cfig := config.Component{
		Name:  "gps1",
		Model: "nmea-pmtkI2C",
		Type:  gps.SubtypeName,
		Attributes: config.AttributeMap{
			"board": "",
			"bus": "",
			"i2c_addr": "",
		},
	}

	logger := golog.NewTestLogger(t)
	ctx := context.Background()

	g, err := newPmtkI2CNMEAGPS(ctx, deps, cfig, logger)
	test.That(t, g, test.ShouldBeNil)
	test.That(t, err, test.ShouldNotBeNil)

	cfig = config.Component{
		Name:  "gps1",
		Model: "nmea-serial",
		Type:  gps.SubtypeName,
		Attributes: config.AttributeMap{
			"board": testBoardName,
			"bus": testBusName,
			"i2c_addr": "",
		},
	}
	g, err = newPmtkI2CNMEAGPS(ctx, deps, cfig, logger)
	passErr := "board " + cfig.Attributes.String("board") + " is not local"
	if err == nil || err.Error() != passErr {
		test.That(t, err, test.ShouldBeNil)
		test.That(t, g, test.ShouldNotBeNil)
	}
}

func TestReadingsI2C(t *testing.T) {
	logger := golog.NewTestLogger(t)
	ctx := context.Background()
	cancelCtx, cancelFunc := context.WithCancel(ctx)
	g := &PmtkI2CNMEAGPS{cancelCtx: cancelCtx, cancelFunc: cancelFunc, logger: logger}
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

	loc1, err := g.ReadLocation(ctx)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, loc1, test.ShouldEqual, loc)

	alt1, err := g.ReadAltitude(ctx)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, alt1, test.ShouldEqual, alt)

	speed1, err := g.ReadSpeed(ctx)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, speed1, test.ShouldEqual, speed)

	inUse, inView, err := g.ReadSatellites(ctx)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, inUse, test.ShouldEqual, activeSats)
	test.That(t, inView, test.ShouldEqual, totalSats)

	acc1, acc2, err := g.ReadAccuracy(ctx)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, acc1, test.ShouldEqual, hAcc)
	test.That(t, acc2, test.ShouldEqual, vAcc)

	valid1, err := g.ReadValid(ctx)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, valid1, test.ShouldEqual, valid)

	fix1, err := g.ReadFix(ctx)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, fix1, test.ShouldEqual, fix)

	readings, err := g.GetReadings(ctx)
	correctReadings := []interface{}{loc.Lat(), loc.Lng(), alt, speed, activeSats, totalSats, hAcc, vAcc, valid, fix}
	test.That(t, err, test.ShouldBeNil)
	test.That(t, readings, test.ShouldResemble, correctReadings)
}

func TestCloseI2C(t *testing.T) {
	logger := golog.NewTestLogger(t)
	ctx := context.Background()
	cancelCtx, cancelFunc := context.WithCancel(ctx)
	g := &PmtkI2CNMEAGPS{cancelCtx: cancelCtx, cancelFunc: cancelFunc, logger: logger}

	err := g.Close()
	test.That(t, err, test.ShouldBeNil)
}