package gpsnmea

import (
	"context"
	"testing"

	"github.com/adrianmo/go-nmea"
	geo "github.com/kellydunn/golang-geo"
	"go.viam.com/test"
	"go.viam.com/utils"

	"go.viam.com/rdk/components/movementsensor"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
	rutils "go.viam.com/rdk/utils"
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
	fakecfg := &SerialConfig{}
	path := "path"
	err := fakecfg.validateSerial(path)
	test.That(t, err, test.ShouldBeError, utils.NewConfigValidationFieldRequiredError(path, "serial_path"))

	fakecfg.SerialPath = "some-path"
	err = fakecfg.validateSerial(path)
	test.That(t, err, test.ShouldBeNil)
}

func TestNewSerialMovementSensor(t *testing.T) {
	var deps resource.Dependencies
	path := "somepath"

	cfig := resource.Config{
		Name:  "movementsensor1",
		Model: resource.DefaultModelFamily.WithModel("gps-nmea"),
		API:   movementsensor.API,
		Attributes: rutils.AttributeMap{
			"path":            "",
			"correction_path": "",
		},
	}

	logger := logging.NewTestLogger(t)
	ctx := context.Background()

	g, err := newNMEAGPS(ctx, deps, cfig, logger)
	test.That(t, g, test.ShouldBeNil)
	test.That(t, err, test.ShouldNotBeNil)

	cfig = resource.Config{
		Name:  "movementsensor1",
		Model: resource.DefaultModelFamily.WithModel("gps-nmea"),
		API:   movementsensor.API,
		ConvertedAttributes: &Config{
			ConnectionType: "serial",
			DisableNMEA:    false,
			SerialConfig: &SerialConfig{
				SerialPath:     path,
				SerialBaudRate: 0,
			},
		},
	}
	g, err = newNMEAGPS(ctx, deps, cfig, logger)
	passErr := "open " + path + ": no such file or directory"
	if err == nil || err.Error() != passErr {
		test.That(t, err, test.ShouldBeNil)
		test.That(t, g, test.ShouldNotBeNil)
	}
}

func TestReadingsSerial(t *testing.T) {
	logger := logging.NewTestLogger(t)
	ctx := context.Background()
	cancelCtx, cancelFunc := context.WithCancel(ctx)
	g := &SerialNMEAMovementSensor{
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

	path := "somepath"
	g.correctionPath = path
	g.correctionBaudRate = 9600
	correctionPath, correctionBaudRate := g.GetCorrectionInfo()
	test.That(t, correctionPath, test.ShouldEqual, path)
	test.That(t, correctionBaudRate, test.ShouldEqual, 9600)

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

func TestCloseSerial(t *testing.T) {
	logger := logging.NewTestLogger(t)
	ctx := context.Background()
	cancelCtx, cancelFunc := context.WithCancel(ctx)
	g := &SerialNMEAMovementSensor{
		cancelCtx:  cancelCtx,
		cancelFunc: cancelFunc,
		logger:     logger,
	}

	err := g.Close(ctx)
	test.That(t, err, test.ShouldBeNil)
}

func TestToPoint(t *testing.T) {
	a := nmea.GLL{Longitude: loc.Lng(), Latitude: loc.Lat()}

	point := toPoint(a)
	test.That(t, point.Lng(), test.ShouldEqual, loc.Lng())
	test.That(t, point.Lat(), test.ShouldEqual, loc.Lat())
}
