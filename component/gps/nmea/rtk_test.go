package nmea

import (
	"context"
	"testing"

	"github.com/edaniels/golog"
	"go.viam.com/test"

	"go.viam.com/rdk/component/board"
	"go.viam.com/rdk/component/gps"
	"go.viam.com/rdk/config"
)

func TestValidateRTK(t *testing.T) {
	fakecfg := &RTKAttrConfig{}
	err := fakecfg.ValidateRTK("path")
	test.That(t, err.Error(), test.ShouldContainSubstring, "expected nonempty ntrip address")

	fakecfg.NtripAddr = "http://fakeurl"
	err = fakecfg.ValidateRTK("path")
	test.That(t, err.Error(), test.ShouldContainSubstring, "expected either nonempty ntrip path, serial path, or I2C board, bus, and address")

	fakecfg.NtripPath = "some-ntrip-path"
	err = fakecfg.ValidateRTK("path")
	test.That(t, err, test.ShouldBeNil)
}

func TestConnect(t *testing.T) {
	logger := golog.NewTestLogger(t)
	ctx := context.Background()
	cancelCtx, cancelFunc := context.WithCancel(ctx)
	g := RTKGPS{cancelCtx: cancelCtx, cancelFunc: cancelFunc, logger: logger}
	url := "http://fakeurl"
	username := "user"
	password := "pwd"
	mountPoint := "mp"

	// create new ntrip client and connect
	err := g.Connect("invalidurl", username, password, 10)
	test.That(t, err, test.ShouldNotBeNil)

	err = g.Connect(url, username, password, 10)
	test.That(t, err, test.ShouldBeNil)

	err = g.GetStream("", 10)
	test.That(t, err, test.ShouldNotBeNil)

	err = g.GetStream(mountPoint, 10)
	test.That(t, err.Error(), test.ShouldContainSubstring, "lookup fakeurl")
}

func TestNewRTKGPS(t *testing.T) {
	path := "somepath"
	deps := setupDependencies(t)

	// serial protocol
	cfig := config.Component{
		Name:  "gps1",
		Model: "rtk",
		Type:  gps.SubtypeName,
		Attributes: config.AttributeMap{
			"ntrip_addr":             "some_ntrip_address",
			"ntrip_username":         "",
			"ntrip_password":         "",
			"ntrip_mountpoint":       "",
			"ntrip_path":             "",
			"ntrip_baud":             115200,
			"ntrip_send_nmea":        true,
			"ntrip_connect_attempts": 10,
			"ntrip_input_protocol":   "serial",
			"path":                   path,
		},
	}

	logger := golog.NewTestLogger(t)
	ctx := context.Background()

	g, err := newRTKGPS(ctx, deps, cfig, logger)
	passErr := "open " + path + ": no such file or directory"
	if err == nil || err.Error() != passErr {
		test.That(t, err, test.ShouldBeNil)
		test.That(t, g, test.ShouldNotBeNil)
	}

	// I2C protocol
	cfig = config.Component{
		Name:  "gps1",
		Model: "rtk",
		Type:  gps.SubtypeName,
		Attributes: config.AttributeMap{
			"ntrip_addr":             "some_ntrip_address",
			"i2c_addr":               "",
			"ntrip_username":         "",
			"ntrip_password":         "",
			"ntrip_mountpoint":       "",
			"ntrip_path":             "",
			"ntrip_baud":             115200,
			"ntrip_send_nmea":        true,
			"ntrip_connect_attempts": 10,
			"ntrip_input_protocol":   "I2C",
			"path":                   path,
			"board":                  testBoardName,
			"bus":                    testBusName,
		},
	}

	logger = golog.NewTestLogger(t)
	ctx = context.Background()

	g, err = newRTKGPS(ctx, deps, cfig, logger)
	passErr = "board " + cfig.Attributes.String("board") + " is not local"

	if err == nil || err.Error() != passErr {
		test.That(t, err, test.ShouldBeNil)
		test.That(t, g, test.ShouldNotBeNil)
	}

	// invalid protocol
	cfig = config.Component{
		Name:  "gps1",
		Model: "rtk",
		Type:  gps.SubtypeName,
		Attributes: config.AttributeMap{
			"ntrip_addr":             "some_ntrip_address",
			"ntrip_username":         "",
			"ntrip_password":         "",
			"ntrip_mountpoint":       "",
			"ntrip_path":             "",
			"ntrip_baud":             115200,
			"ntrip_send_nmea":        true,
			"ntrip_connect_attempts": 10,
			"ntrip_input_protocol":   "notserial",
			"path":                   path,
		},
	}

	logger = golog.NewTestLogger(t)
	ctx = context.Background()

	_, err = newRTKGPS(ctx, deps, cfig, logger)
	test.That(t, err, test.ShouldNotBeNil)

	// No ntrip address
	cfig = config.Component{
		Name:  "gps1",
		Model: "rtk",
		Type:  gps.SubtypeName,
		Attributes: config.AttributeMap{
			"ntrip_addr":             "some_ntrip_address",
			"ntrip_username":         "",
			"ntrip_password":         "",
			"ntrip_mountpoint":       "",
			"ntrip_path":             "",
			"ntrip_baud":             115200,
			"ntrip_send_nmea":        true,
			"ntrip_connect_attempts": 10,
			"ntrip_input_protocol":   "serial",
			"path":                   path,
		},
	}

	logger = golog.NewTestLogger(t)
	ctx = context.Background()

	_, err = newRTKGPS(ctx, deps, cfig, logger)
	test.That(t, err, test.ShouldNotBeNil)
}

func TestReadingsRTK(t *testing.T) {
	logger := golog.NewTestLogger(t)
	ctx := context.Background()
	cancelCtx, cancelFunc := context.WithCancel(ctx)
	g := RTKGPS{cancelCtx: cancelCtx, cancelFunc: cancelFunc, logger: logger}
	nmeagps := &SerialNMEAGPS{cancelCtx: cancelCtx, cancelFunc: cancelFunc, logger: logger}
	nmeagps.data = gpsData{
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
	g.nmeagps = nmeagps

	status, err := g.NtripStatus()
	test.That(t, err, test.ShouldBeNil)
	test.That(t, status, test.ShouldEqual, false)

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

func TestCloseRTK(t *testing.T) {
	logger := golog.NewTestLogger(t)
	ctx := context.Background()
	cancelCtx, cancelFunc := context.WithCancel(ctx)
	g := RTKGPS{cancelCtx: cancelCtx, cancelFunc: cancelFunc, logger: logger}
	g.nmeagps = &SerialNMEAGPS{cancelCtx: cancelCtx, cancelFunc: cancelFunc, logger: logger}

	err := g.Close()
	test.That(t, err, test.ShouldBeNil)
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
