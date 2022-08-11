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

func TestValidate(t *testing.T) {
	fakecfg := &AttrConfig{}
	err := fakecfg.Validate("path")
	test.That(t, err.Error(), test.ShouldContainSubstring, "expected nonempty correction source")

	fakecfg.CorrectionSource = "notvalid"
	err = fakecfg.Validate("path")
	test.That(t, err.Error(), test.ShouldContainSubstring, "only serial, I2C, and ntrip are supported correction sources")

	// ntrip
	fakecfg.CorrectionSource = "ntrip"
	err = fakecfg.Validate("path")
	test.That(t, err.Error(), test.ShouldContainSubstring, "expected nonempty ntrip address")

	fakecfg.NtripAddr = "some-ntrip-address"
	err = fakecfg.Validate("path")
	test.That(t, err, test.ShouldBeNil)

	// serial
	fakecfg.CorrectionSource = "serial"
	err = fakecfg.Validate("path")
	test.That(t, err.Error(), test.ShouldContainSubstring, "must specify serial path")

	fakecfg.CorrectionPath = "some-serial-path"
	err = fakecfg.Validate("path")
	test.That(t, err, test.ShouldBeNil)

	// I2C
	fakecfg.CorrectionSource = "I2C"
	err = fakecfg.Validate("path")
	test.That(t, err.Error(), test.ShouldContainSubstring, "cannot find board for rtk station")
}

func TestRTK(t *testing.T) {
	logger := golog.NewTestLogger(t)
	ctx := context.Background()
	deps := setupDependencies(t)

	// test NTRIPConnection Source
	cfig := config.Component{
		Name:  "rtk1",
		Model: "rtk-station",
		Type:  "gps",
		Attributes: config.AttributeMap{
			"correction_source":      "ntrip",
			"ntrip_addr":             "some_ntrip_address",
			"ntrip_username":         "",
			"ntrip_password":         "",
			"ntrip_mountpoint":       "NJI2",
			"ntrip_connect_attempts": 10,
		},
	}

	g, err := newRTKStation(ctx, deps, cfig, logger)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, g, test.ShouldNotBeNil)

	// test serial connection source
	path := "/dev/serial/by-id/usb-u-blox_AG_-_www.u-blox.com_u-blox_GNSS_receiver-if00"
	cfig = config.Component{
		Name:  "rtk1",
		Model: "rtk-station",
		Type:  "gps",
		Attributes: config.AttributeMap{
			"correction_source": "serial",
			"correction_path":   path,
		},
	}

	g, err = newRTKStation(ctx, deps, cfig, logger)
	passErr := "open " + path + ": no such file or directory"
	if err == nil || err.Error() != passErr {
		test.That(t, err, test.ShouldBeNil)
		test.That(t, g, test.ShouldNotBeNil)
	}

	// test I2C correction source
	cfig = config.Component{
		Name:  "rtk1",
		Model: "rtk-station",
		Type:  "gps",
		Attributes: config.AttributeMap{
			"correction_source":      "I2C",
			"ntrip_addr":             "some_ntrip_address",
			"ntrip_username":         "",
			"ntrip_password":         "",
			"ntrip_mountpoint":       "NJI2",
			"ntrip_connect_attempts": 10,
			"board":                  testBoardName,
			"bus":                    testBusName,
		},
	}

	g, err = newRTKStation(ctx, deps, cfig, logger)
	passErr = "board " + cfig.Attributes.String("board") + " is not local"

	if err == nil || err.Error() != passErr {
		test.That(t, err, test.ShouldBeNil)
		test.That(t, g, test.ShouldNotBeNil)
	}

	// test invalid source
	cfig = config.Component{
		Name:  "rtk1",
		Model: "rtk-station",
		Type:  "gps",
		Attributes: config.AttributeMap{
			"correction_source":      "invalid-protocol",
			"ntrip_addr":             "some_ntrip_address",
			"ntrip_username":         "",
			"ntrip_password":         "",
			"ntrip_mountpoint":       "NJI2",
			"ntrip_connect_attempts": 10,
			"children":               "gps1",
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
	g := rtkStation{cancelCtx: cancelCtx, cancelFunc: cancelFunc, logger: logger}
	r := ioutil.NopCloser(strings.NewReader("hello world"))
	n := &ntripCorrectionSource{cancelCtx: cancelCtx, cancelFunc: cancelFunc, logger: logger, correctionReader: r}
	g.correction = n

	err := g.Close()
	test.That(t, err, test.ShouldBeNil)

	g = rtkStation{cancelCtx: cancelCtx, cancelFunc: cancelFunc, logger: logger}
	s := &serialCorrectionSource{cancelCtx: cancelCtx, cancelFunc: cancelFunc, logger: logger, correctionReader: r}
	g.correction = s

	err = g.Close()
	test.That(t, err, test.ShouldBeNil)

	g = rtkStation{cancelCtx: cancelCtx, cancelFunc: cancelFunc, logger: logger}
	i := &i2cCorrectionSource{cancelCtx: cancelCtx, cancelFunc: cancelFunc, logger: logger, correctionReader: r}
	g.correction = i

	err = g.Close()
	test.That(t, err, test.ShouldBeNil)
}

func TestConnect(t *testing.T) {
	logger := golog.NewTestLogger(t)
	ctx := context.Background()
	cancelCtx, cancelFunc := context.WithCancel(ctx)
	info := ntripInfo{
		url:                "invalidurl",
		username:           "user",
		password:           "pwd",
		mountPoint:         "",
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
	test.That(t, err.Error(), test.ShouldContainSubstring, "lookup fakeurl")
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
