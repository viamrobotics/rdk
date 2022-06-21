package nmea

import (
	"context"
	"strings"
	"testing"

	"github.com/edaniels/golog"
	geo "github.com/kellydunn/golang-geo"
	"go.viam.com/test"

	"go.viam.com/rdk/component/gps"
	"go.viam.com/rdk/config"
)

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
	if !strings.Contains(err.Error(), "no such host") {
		t.Error()
	}
}

func TestNewRTKGPS(t *testing.T) {
	path := "somepath"

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

	g, err := newRTKGPS(ctx, cfig, logger)
	passErr := "open " + path + ": no such file or directory"
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

	_, err = newRTKGPS(ctx, cfig, logger)
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

	_, err = newRTKGPS(ctx, cfig, logger)
	test.That(t, err, test.ShouldNotBeNil)
}

var (
	loc        = geo.NewPoint(90, 1)
	alt        = 50.5
	speed      = 5.4
	activeSats = 1
	totalSats  = 2
	hAcc       = 0.7
	vAcc       = 0.8
	valid      = true
)

func TestReadings(t *testing.T) {
	logger := golog.NewTestLogger(t)
	ctx := context.Background()
	cancelCtx, cancelFunc := context.WithCancel(ctx)
	g := RTKGPS{cancelCtx: cancelCtx, cancelFunc: cancelFunc, logger: logger}
	nmeagps := &serialNMEAGPS{cancelCtx: cancelCtx, cancelFunc: cancelFunc, logger: logger}
	nmeagps.data = gpsData{
		location:   loc,
		alt:        alt,
		speed:      speed,
		vDOP:       vAcc,
		hDOP:       hAcc,
		satsInView: totalSats,
		satsInUse:  activeSats,
		valid:      valid,
	}
	g.nmeagps = nmeagps

	status, err := g.NtripStatus()
	test.That(t, err, test.ShouldBeNil)
	test.That(t, status, test.ShouldNotBeNil)

	loc, err := g.ReadLocation(ctx)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, loc, test.ShouldNotBeNil)

	alt, err := g.ReadAltitude(ctx)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, alt, test.ShouldNotBeNil)

	speed, err := g.ReadSpeed(ctx)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, speed, test.ShouldNotBeNil)

	inUse, inView, err := g.ReadSatellites(ctx)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, inUse, test.ShouldNotBeNil)
	test.That(t, inView, test.ShouldNotBeNil)

	acc1, acc2, err := g.ReadAccuracy(ctx)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, acc1, test.ShouldNotBeNil)
	test.That(t, acc2, test.ShouldNotBeNil)

	valid, err := g.ReadValid(ctx)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, valid, test.ShouldNotBeNil)
}

func TestClose(t *testing.T) {
	logger := golog.NewTestLogger(t)
	ctx := context.Background()
	cancelCtx, cancelFunc := context.WithCancel(ctx)
	g := RTKGPS{cancelCtx: cancelCtx, cancelFunc: cancelFunc, logger: logger}
	g.nmeagps = &serialNMEAGPS{cancelCtx: cancelCtx, cancelFunc: cancelFunc, logger: logger}

	err := g.Close()
	test.That(t, err, test.ShouldBeNil)
}
