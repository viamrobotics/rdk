package rtk

import (
	"context"
	"io/ioutil"
	"strings"
	"testing"

	"github.com/edaniels/golog"
	"go.viam.com/test"

	"go.viam.com/rdk/component/board"
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

func TestRTK(t *testing.T) {
	logger := golog.NewTestLogger(t)
	ctx := context.Background()
	deps := setupDependencies(t)

	//test NTRIPConnection Source
	cfig := config.Component{
		Name:  "rtk1",
		Model: "rtk-station",
		Type:  "gps",
		Attributes: config.AttributeMap{
			"correction_source": "ntrip",
			"ntrip_addr": "some_ntrip_address",
			"ntrip_username": "skarpoor",
			"ntrip_password": "plswork",
			"ntrip_mountpoint": "NJI2",
			"ntrip_connect_attempts": 10,
		},
	}

	g, err := newRTKStation(ctx, deps, cfig, logger)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, g, test.ShouldNotBeNil)

	// test serial connection source
	cfig = config.Component{
		Name:  "rtk1",
		Model: "rtk-station",
		Type:  "gps",
		Attributes: config.AttributeMap{
			"correction_source": "serial",
			"correction_path": "/dev/serial/by-id/usb-u-blox_AG_-_www.u-blox.com_u-blox_GNSS_receiver-if00",
		},
	}
	g, err = newRTKStation(ctx, deps, cfig, logger)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, g, test.ShouldNotBeNil)

	//test I2C correction source
	cfig = config.Component{
		Name:  "rtk1",
		Model: "rtk-station",
		Type:  "gps",
		Attributes: config.AttributeMap{
			"correction_source": "I2C",
			"ntrip_addr": "some_ntrip_address",
			"ntrip_username": "skarpoor",
			"ntrip_password": "plswork",
			"ntrip_mountpoint": "NJI2",
			"ntrip_connect_attempts": 10,
			"board":                  testBoardName,
			"bus":                    testBusName,
		},
	}

	g, err = newRTKStation(ctx, deps, cfig, logger)
	passErr := "board " + cfig.Attributes.String("board") + " is not local"

	if err == nil || err.Error() != passErr {
		test.That(t, err, test.ShouldBeNil)
		test.That(t, g, test.ShouldNotBeNil)
	}

	//test invalid source
	cfig = config.Component{
		Name:  "rtk1",
		Model: "rtk-station",
		Type:  "gps",
		Attributes: config.AttributeMap{
			"correction_source": "invalid-protocol",
			"ntrip_addr": "some_ntrip_address",
			"ntrip_username": "skarpoor",
			"ntrip_password": "plswork",
			"ntrip_mountpoint": "NJI2",
			"ntrip_connect_attempts": 10,
			"board":                  testBoardName,
			"bus":                    testBusName,
		},
	}
	_, err = newRTKStation(ctx, deps, cfig, logger)
	test.That(t, err, test.ShouldNotBeNil)
}

func TestClose(t *testing.T) {
	logger := golog.NewTestLogger(t)
	ctx := context.Background()
	cancelCtx, cancelFunc := context.WithCancel(ctx)
	g := RTKStation{cancelCtx: cancelCtx, cancelFunc: cancelFunc, logger: logger}
	r := ioutil.NopCloser(strings.NewReader("hello world"))
	n := &ntripCorrectionSource{cancelCtx: cancelCtx, cancelFunc: cancelFunc, logger:logger, correctionReader: r}
	g.correction = n

	err := g.Close()
	test.That(t, err, test.ShouldBeNil)

	g = RTKStation{cancelCtx: cancelCtx, cancelFunc: cancelFunc, logger: logger}
	s := &serialCorrectionSource{cancelCtx: cancelCtx, cancelFunc: cancelFunc, logger:logger, correctionReader: r}
	g.correction = s

	err = g.Close()
	test.That(t, err, test.ShouldBeNil)

	g = RTKStation{cancelCtx: cancelCtx, cancelFunc: cancelFunc, logger: logger}
	i := &i2cCorrectionSource{cancelCtx: cancelCtx, cancelFunc: cancelFunc, logger:logger, correctionReader: r}
	g.correction = i

	err = g.Close()
	test.That(t, err, test.ShouldBeNil)
}
func TestReadings(t *testing.T) {
	logger := golog.NewTestLogger(t)
	ctx := context.Background()
	cancelCtx, cancelFunc := context.WithCancel(ctx)
	g := RTKStation{cancelCtx: cancelCtx, cancelFunc: cancelFunc, logger: logger}

	loc1, err := g.ReadLocation(ctx)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, loc1, test.ShouldBeNil)

	alt1, err := g.ReadAltitude(ctx)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, alt1, test.ShouldEqual, 0)

	speed1, err := g.ReadSpeed(ctx)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, speed1, test.ShouldEqual, 0)

	inUse, inView, err := g.ReadSatellites(ctx)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, inUse, test.ShouldEqual, 0)
	test.That(t, inView, test.ShouldEqual, 0)

	acc1, acc2, err := g.ReadAccuracy(ctx)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, acc1, test.ShouldEqual, 0)
	test.That(t, acc2, test.ShouldEqual, 0)

	valid1, err := g.ReadValid(ctx)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, valid1, test.ShouldEqual, false)
}

func TestConnect(t *testing.T) {
	logger := golog.NewTestLogger(t)
	ctx := context.Background()
	cancelCtx, cancelFunc := context.WithCancel(ctx)
	info := ntripInfo{
		url: "invalidurl",
		username: "user",
		password:"pwd",
		mountPoint: "",
		maxConnectAttempts: 10,
	}
	g := &ntripCorrectionSource{cancelCtx: cancelCtx, cancelFunc: cancelFunc, logger: logger, info: info}

	// create new ntrip client and connect
	err := g.Connect()
	test.That(t, err, test.ShouldNotBeNil)

	g.info.url = "http://fakeurl"
	err = g.Connect()
	test.That(t, err, test.ShouldBeNil)

	err = g.GetStream()
	test.That(t, err, test.ShouldNotBeNil)

	g.info.mountPoint = "mp"
	err = g.GetStream()
	test.That(t, err.Error(), test.ShouldContainSubstring, "no such host")
}

// Helpers

type mock struct {
	board.LocalBoard
	Name string

	i2cs []string
	i2c  *mockI2C
}

func newBoard(name string) *mock {
	return &mock{
		Name: name,
		i2cs: []string{"i2c1"},
		i2c:  &mockI2C{1},
	}
}

// Mock I2C

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

