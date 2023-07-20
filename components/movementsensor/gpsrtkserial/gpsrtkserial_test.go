package gpsrtkserial

import (
	"context"
	"testing"

	"github.com/edaniels/golog"
	geo "github.com/kellydunn/golang-geo"
	"go.viam.com/rdk/components/board"
	"go.viam.com/rdk/components/movementsensor"
	"go.viam.com/rdk/components/movementsensor/fake"
	gpsnmea "go.viam.com/rdk/components/movementsensor/gpsnmea"
	rtk "go.viam.com/rdk/components/movementsensor/rtkutils"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/testutils/inject"
	rutils "go.viam.com/rdk/utils"
	"go.viam.com/test"
	"go.viam.com/utils"
)

var (
	alt   = 50.5
	speed = 5.4
	fix   = 1
)

const (
	testRoverName   = "testRover"
	testStationName = "testStation"
	testBoardName   = "board1"
	testBusName     = "bus1"
	testi2cAddr     = 44
)

// mock ntripinfo client.
func makeMockNtripClient() *rtk.NtripInfo {
	return &rtk.NtripInfo{}
}

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

	conf := resource.Config{
		Name:  "rtk-sensor1",
		Model: resource.DefaultModelFamily.WithModel("gps-nmea"),
		API:   movementsensor.API,
	}

	c := make(chan []byte, 1024)

	serialnmeaConf := &gpsnmea.Config{
		ConnectionType: serialStr,
		SerialConfig: &gpsnmea.SerialConfig{
			SerialPath: "some-path",
			TestChan:   c,
		},
	}

	logger := golog.NewTestLogger(t)
	ctx := context.Background()

	serialNMEA, _ := gpsnmea.NewSerialGPSNMEA(ctx, conf.ResourceName(), serialnmeaConf, logger)

	conf.Name = "rtk-sensor2"

	rtkSensor := &rtkSerial{
		nmeamovementsensor: serialNMEA, InputProtocol: serialStr,
	}

	deps[movementsensor.Named("rtk-sensor")] = rtkSensor

	return deps
}

func setupInjectRobotWithGPS() *inject.Robot {
	r := &inject.Robot{}

	r.ResourceByNameFunc = func(name resource.Name) (resource.Resource, error) {
		switch name {
		case movementsensor.Named(testRoverName):
			return &rtkSerial{}, nil
		default:
			return nil, resource.NewNotFoundError(name)
		}
	}
	r.ResourceNamesFunc = func() []resource.Name {
		return []resource.Name{movementsensor.Named(testRoverName), movementsensor.Named(testStationName)}
	}
	return r
}

func TestValidateRTK(t *testing.T) {
	path := "path"
	fakecfg := &Config{
		NtripURL:             "",
		NtripConnectAttempts: 10,
		NtripPass:            "somepass",
		NtripUser:            "someuser",
		NtripMountpoint:      "NYC",
		SerialPath:           path,
		SerialBaudRate:       3600,
	}
	_, err := fakecfg.Validate(path)
	test.That(
		t,
		err,
		test.ShouldBeError,
		utils.NewConfigValidationFieldRequiredError(path, "ntrip_url"))

	fakecfg.NtripURL = "asdfg"
	_, err = fakecfg.Validate(path)
	test.That(
		t,
		err,
		test.ShouldBeNil)
	_, err = fakecfg.Validate("path")
	test.That(t, err, test.ShouldBeNil)
}

func TestConnect(t *testing.T) {
	logger := golog.NewTestLogger(t)
	ctx := context.Background()
	cancelCtx, cancelFunc := context.WithCancel(ctx)
	g := rtkSerial{
		cancelCtx:  cancelCtx,
		cancelFunc: cancelFunc,
		logger:     logger,
	}

	url := "http://fakeurl"
	username := "user"
	password := "pwd"

	// create new ntrip client and connect
	err := g.connect("invalidurl", username, password, 10)
	g.ntripClient = makeMockNtripClient()

	test.That(t, err, test.ShouldNotBeNil)

	err = g.connect(url, username, password, 10)
	test.That(t, err, test.ShouldBeNil)

	err = g.getStream("", 10)
	test.That(t, err, test.ShouldNotBeNil)
}

func TestReadings(t *testing.T) {
	logger := golog.NewTestLogger(t)
	ctx := context.Background()
	cancelCtx, cancelFunc := context.WithCancel(ctx)
	g := rtkSerial{
		cancelCtx:  cancelCtx,
		cancelFunc: cancelFunc,
		logger:     logger,
	}

	g.nmeamovementsensor = &fake.MovementSensor{}

	status, err := g.getNtripConnectionStatus()
	test.That(t, err, test.ShouldBeNil)
	test.That(t, status, test.ShouldEqual, false)

	loc1, alt1, err := g.Position(ctx, make(map[string]interface{}))
	test.That(t, err, test.ShouldBeNil)
	test.That(t, loc1, test.ShouldResemble, geo.NewPoint(40.7, -73.98))
	test.That(t, alt1, test.ShouldEqual, alt)

	speed1, err := g.LinearVelocity(ctx, make(map[string]interface{}))
	test.That(t, err, test.ShouldBeNil)
	test.That(t, speed1.Y, test.ShouldEqual, speed)

	fix1, err := g.readFix(ctx)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, fix1, test.ShouldEqual, fix)
}

func TestNewRTKSerial(t *testing.T) {
	path := "somepath"
	deps := setupDependencies(t)
	logger := golog.NewTestLogger(t)
	ctx := context.Background()

	t.Run("serial protocol", func(t *testing.T) {
		// serial protocol
		conf := resource.Config{
			Name:  "movementsensor1",
			Model: rtkmodel,
			API:   movementsensor.API,
			Attributes: rutils.AttributeMap{
				"ntrip_send_nmea":           true,
				"ntrip_connect_attempts":    10,
				"correction_input_protocol": "serial",
				"path":                      path,
			},
			ConvertedAttributes: &Config{
				SerialPath:           path,
				SerialBaudRate:       0,
				NtripURL:             "some_ntrip_address",
				NtripConnectAttempts: 10,
				NtripMountpoint:      "",
				NtripPass:            "",
				NtripUser:            "",
			},
		}

		// TODO(RSDK-2698): this test is not really doing anything since it needs a mocked
		// serial; it used to just test a random error; it still does.
		_, err := newRTKSerial(ctx, deps, conf, logger)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "no such file")
	})

	t.Run("invalid protocol", func(t *testing.T) {
		// invalid protocol
		conf := resource.Config{
			Name:  "movementsensor1",
			Model: resource.DefaultModelFamily.WithModel("rtk"),
			API:   movementsensor.API,
			Attributes: rutils.AttributeMap{
				"ntrip_addr":                "some_ntrip_address",
				"ntrip_username":            "",
				"ntrip_password":            "",
				"ntrip_mountpoint":          "",
				"ntrip_path":                "",
				"ntrip_baud":                115200,
				"ntrip_send_nmea":           true,
				"ntrip_connect_attempts":    10,
				"correction_input_protocol": "notserial",
				"path":                      path,
			},
			ConvertedAttributes: &Config{},
		}
		_, err := newRTKSerial(ctx, deps, conf, logger)
		test.That(t, err, test.ShouldNotBeNil)
	})

	t.Run("no ntrip address", func(t *testing.T) {
		conf := resource.Config{
			Name:  "movementsensor1",
			Model: resource.DefaultModelFamily.WithModel("rtk"),
			API:   movementsensor.API,
			Attributes: rutils.AttributeMap{
				"ntrip_addr":                "some_ntrip_address",
				"ntrip_username":            "",
				"ntrip_password":            "",
				"ntrip_mountpoint":          "",
				"ntrip_path":                "",
				"ntrip_baud":                115200,
				"ntrip_send_nmea":           true,
				"ntrip_connect_attempts":    10,
				"correction_input_protocol": "serial",
				"path":                      path,
			},
			ConvertedAttributes: &Config{},
		}

		_, err := newRTKSerial(ctx, deps, conf, logger)
		test.That(t, err, test.ShouldNotBeNil)
	})
}

func TestCloseRTK(t *testing.T) {
	logger := golog.NewTestLogger(t)
	ctx := context.Background()
	cancelCtx, cancelFunc := context.WithCancel(ctx)
	g := rtkSerial{
		cancelCtx:  cancelCtx,
		cancelFunc: cancelFunc,
		logger:     logger,
	}
	g.nmeamovementsensor = &fake.MovementSensor{}

	err := g.Close(ctx)
	test.That(t, err, test.ShouldBeNil)
}
