package rtk

import (
	"context"
	"testing"

	"github.com/edaniels/golog"
	// geo "github.com/kellydunn/golang-geo"
	"go.viam.com/test"
	"go.viam.com/rdk/component/gps/nmea"
	"go.viam.com/rdk/component/gps"

	"go.viam.com/rdk/component/board"
	// "go.viam.com/rdk/component/gps"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/registry"
)

const (
	testBoardName = "board1"
	testBusName   = "i2c1"
	gpsChild 	  = "gps1"
)

func setupDependencies(t *testing.T) registry.Dependencies {
	t.Helper()

	deps := make(registry.Dependencies)

	actualBoard := newBoard(testBoardName)
	deps[board.Named(testBoardName)] = actualBoard

	logger := golog.NewTestLogger(t)
	ctx := context.Background()
	cancelCtx, cancelFunc := context.WithCancel(ctx)
	g := &nmea.SerialNMEAGPS{cancelCtx: cancelCtx, cancelFunc: cancelFunc, logger: logger}

	deps[gps.Named(gpsChild)] = g

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
			"ntrip_username": "",
			"ntrip_password": "",
			"ntrip_mountpoint": "NJI2",
			"ntrip_connect_attempts": 10,
			"children": "gps1",
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
			"ntrip_username": "",
			"ntrip_password": "",
			"ntrip_mountpoint": "NJI2",
			"ntrip_connect_attempts": 10,
			"board":                  testBoardName,
			"bus":                    testBusName,
			"children": "gps1",
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
			"ntrip_username": "",
			"ntrip_password": "",
			"ntrip_mountpoint": "NJI2",
			"ntrip_connect_attempts": 10,
			"children": "gps1",
			"board":                  testBoardName,
			"bus":                    testBusName,
		},
	}
	_, err = newRTKStation(ctx, deps, cfig, logger)
	test.That(t, err, test.ShouldNotBeNil)
}

func TestClose(t *testing.T) {

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

