package nmea

import (
	"context"
	"testing"

	"github.com/adrianmo/go-nmea"
	"github.com/edaniels/golog"
	geo "github.com/kellydunn/golang-geo"
	"go.viam.com/test"

	"go.viam.com/rdk/component/gps"
	"go.viam.com/rdk/config"
)

var (
	loc        = geo.NewPoint(90, 1)
	alt        = 50.5
	speed      = 5.4
	activeSats = 1
	totalSats  = 2
	hAcc       = 0.7
	vAcc       = 0.8
	valid      = true
	fix        = 1
)

func TestValidateSerial(t *testing.T) {
	fakecfg := &SerialAttrConfig{}
	err := fakecfg.ValidateSerial("path")
	test.That(t, err.Error(), test.ShouldContainSubstring, "expected nonempty path")

	fakecfg.SerialPath = "some-path"
	err = fakecfg.ValidateSerial("path")
	test.That(t, err, test.ShouldBeNil)
}

func TestNewSerialGPS(t *testing.T) {
	path := "somepath"

	cfig := config.Component{
		Name:  "gps1",
		Model: "nmea-serial",
		Type:  gps.SubtypeName,
		Attributes: config.AttributeMap{
			"path":            "",
			"correction_path": "",
		},
	}

	logger := golog.NewTestLogger(t)
	ctx := context.Background()

	g, err := newSerialNMEAGPS(ctx, cfig, logger)
	test.That(t, g, test.ShouldBeNil)
	test.That(t, err, test.ShouldNotBeNil)

	cfig = config.Component{
		Name:  "gps1",
		Model: "nmea-serial",
		Type:  gps.SubtypeName,
		Attributes: config.AttributeMap{
			"path":            path,
			"correction_path": "",
		},
	}
	g, err = newSerialNMEAGPS(ctx, cfig, logger)
	passErr := "open " + path + ": no such file or directory"
	if err == nil || err.Error() != passErr {
		test.That(t, err, test.ShouldBeNil)
		test.That(t, g, test.ShouldNotBeNil)
	}
}

func TestReadingsSerial(t *testing.T) {
	logger := golog.NewTestLogger(t)
	ctx := context.Background()
	cancelCtx, cancelFunc := context.WithCancel(ctx)
	g := &SerialNMEAGPS{cancelCtx: cancelCtx, cancelFunc: cancelFunc, logger: logger}
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

	path := "somepath"
	g.correctionPath = path
	g.correctionBaudRate = 9600

	correctionPath, correctionBaudRate := g.GetCorrectionInfo()
	test.That(t, correctionPath, test.ShouldEqual, path)
	test.That(t, correctionBaudRate, test.ShouldEqual, 9600)

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

func TestCloseSerial(t *testing.T) {
	logger := golog.NewTestLogger(t)
	ctx := context.Background()
	cancelCtx, cancelFunc := context.WithCancel(ctx)
	g := &SerialNMEAGPS{cancelCtx: cancelCtx, cancelFunc: cancelFunc, logger: logger}

	err := g.Close()
	test.That(t, err, test.ShouldBeNil)
}

func TestToPoint(t *testing.T) {
	a := nmea.GLL{Longitude: loc.Lng(), Latitude: loc.Lat()}

	point := toPoint(a)
	test.That(t, point.Lng(), test.ShouldEqual, loc.Lng())
	test.That(t, point.Lat(), test.ShouldEqual, loc.Lat())
}
