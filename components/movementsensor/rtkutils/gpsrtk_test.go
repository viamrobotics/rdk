package rtkutils

import (
	"context"
	"testing"

	geo "github.com/kellydunn/golang-geo"
	"go.viam.com/test"

	"go.viam.com/rdk/components/movementsensor"
	"go.viam.com/rdk/components/movementsensor/fake"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/testutils/inject"
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

// TODO: RSDK-3264, This needs to be cleaned up as we stablize gpsrtk
/* func setupDependencies(t *testing.T) resource.Dependencies {
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

	i2cnmeaConf := &gpsnmea.Config{
		ConnectionType: i2cStr,
		Board:          testBoardName,
		I2CConfig: &gpsnmea.I2CConfig{
			I2CBus:  testBusName,
			I2cAddr: testi2cAddr,
		},
	}

	logger := logging.NewTestLogger(t)
	ctx := context.Background()

	serialNMEA, _ := gpsnmea.NewSerialGPSNMEA(ctx, conf.ResourceName(), serialnmeaConf, logger)

	conf.Name = "rtk-sensor2"
	i2cNMEA, _ := gpsnmea.NewPmtkI2CGPSNMEA(ctx, deps, conf.ResourceName(), i2cnmeaConf, logger)

	rtkSensor1 := &RTKMovementSensor{
		Nmeamovementsensor: serialNMEA, InputProtocol: serialStr,
	}

	rtkSensor2 := &RTKMovementSensor{
		Nmeamovementsensor: i2cNMEA, InputProtocol: i2cStr,
	}

	deps[movementsensor.Named("rtk-sensor1")] = rtkSensor1
	deps[movementsensor.Named("rtk-sensor2")] = rtkSensor2

	return deps
} */

func setupInjectRobotWithGPS() *inject.Robot {
	r := &inject.Robot{}

	r.ResourceByNameFunc = func(name resource.Name) (resource.Resource, error) {
		switch name {
		case movementsensor.Named(testRoverName):
			return &RTKMovementSensor{}, nil
		default:
			return nil, resource.NewNotFoundError(name)
		}
	}
	r.ResourceNamesFunc = func() []resource.Name {
		return []resource.Name{movementsensor.Named(testRoverName), movementsensor.Named(testStationName)}
	}
	return r
}

func TestModelTypeCreators(t *testing.T) {
	r := setupInjectRobotWithGPS()
	gps1, err := movementsensor.FromRobot(r, testRoverName)
	test.That(t, gps1, test.ShouldResemble, &RTKMovementSensor{})
	test.That(t, err, test.ShouldBeNil)
}

// func TestValidateRTK(t *testing.T) {
// 	path := "path"
// 	fakecfg := &Config{NtripConfig: &NtripConfig{}, ConnectionType: "serial", SerialConfig: &SerialConfig{SerialPath: "some-path"}}
// 	_, err := fakecfg.Validate(path)

// 	test.That(t, err, test.ShouldBeError,
// 		utils.NewConfigValidationFieldRequiredError(path, "correction_source"))

// 	fakecfg.CorrectionSource = "ntrip"
// 	_, err = fakecfg.Validate(path)
// 	test.That(t, err, test.ShouldBeError, utils.NewConfigValidationFieldRequiredError(path, "ntrip_url"))

// 	fakecfg.NtripConfig.NtripURL = "http://fakeurl"
// 	_, err = fakecfg.Validate(path)
// 	test.That(
// 		t,
// 		err,
// 		test.ShouldBeError,
// 		utils.NewConfigValidationFieldRequiredError(path, "ntrip_input_protocol"),
// 	)
// 	fakecfg.NtripInputProtocol = "serial"
// 	_, err = fakecfg.Validate("path")
// 	test.That(t, err, test.ShouldBeNil)
// }

// func TestConnect(t *testing.T) {
// 	logger := logging.NewTestLogger(t)
// 	ctx := context.Background()
// 	cancelCtx, cancelFunc := context.WithCancel(ctx)
// 	g := RTKMovementSensor{
// 		cancelCtx:  cancelCtx,
// 		cancelFunc: cancelFunc,
// 		logger:     logger,
// 	}

// 	url := "http://fakeurl"
// 	username := "user"
// 	password := "pwd"

// 	// create new ntrip client and connect
// 	err := g.Connect("invalidurl", username, password, 10)
// 	g.ntripClient = makeMockNtripClient()

// 	test.That(t, err, test.ShouldNotBeNil)

// 	err = g.Connect(url, username, password, 10)
// 	test.That(t, err, test.ShouldBeNil)

// 	err = g.GetStream("", 10)
// 	test.That(t, err, test.ShouldNotBeNil)
// }

// TODO: RSDK-3264, This needs to be cleaned up as we stablize gpsrtk
/* func TestNewRTKMovementSensor(t *testing.T) {
	path := "somepath"
	deps := setupDependencies(t)
	logger := logging.NewTestLogger(t)
	ctx := context.Background()

	t.Run("serial protocol", func(t *testing.T) {
		// serial protocol
		conf := resource.Config{
			Name:  "movementsensor1",
			Model: roverModel,
			API:   movementsensor.API,
			Attributes: rutils.AttributeMap{
				"ntrip_send_nmea":           true,
				"ntrip_connect_attempts":    10,
				"correction_input_protocol": "serial",
				"path":                      path,
			},
			ConvertedAttributes: &Config{
				CorrectionSource: "serial",
				ConnectionType:   "serial",
				SerialConfig: &SerialConfig{
					SerialPath:               path,
					SerialBaudRate:           0,
					SerialCorrectionPath:     path,
					SerialCorrectionBaudRate: 0,
				},
				NtripConfig: &NtripConfig{
					NtripURL:            "some_ntrip_address",
					NtripConnectAttempts: 10,
					NtripMountpoint:      "",
					NtripPass:            "",
					NtripUser:            "",
					NtripPath:            path,
					NtripBaud:            115200,
					NtripInputProtocol:   "serial",
				},
			},
		}

		// TODO(RSDK-2698): this test is not really doing anything since it needs a mocked
		// serial; it used to just test a random error; it still does.
		_, err := newRTKMovementSensor(ctx, deps, conf, logger)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "no such file")
	})

	t.Run("I2C protocol", func(t *testing.T) {
		conf := resource.Config{
			Name:  "movementsensor1",
			Model: roverModel,
			API:   movementsensor.API,
			Attributes: rutils.AttributeMap{
				"ntrip_addr":                "some_ntrip_address",
				"i2c_addr":                  "",
				"ntrip_username":            "",
				"ntrip_password":            "",
				"ntrip_mountpoint":          "",
				"ntrip_path":                "",
				"ntrip_baud":                115200,
				"ntrip_send_nmea":           true,
				"ntrip_connect_attempts":    10,
				"correction_input_protocol": "i2c",
				"path":                      path,
				"board":                     testBoardName,
				"bus":                       testBusName,
			},
			ConvertedAttributes: &Config{
				CorrectionSource: "i2c",
				ConnectionType:   "i2c",
				I2CConfig: &I2CConfig{
					Board:       testBoardName,
					I2CBus:      testBusName,
					I2cAddr:     0,
					I2CBaudRate: 115200,
				},
				NtripConfig: &NtripConfig{
					NtripURL: "http://some_ntrip_address",
				},
			},
		}

		// TODO(RSDK-2698): this test is not really doing anything since it needs a mocked
		// I2C; it used to just test a random error; it still does.
		g, err := newRTKMovementSensor(ctx, deps, conf, logger)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, g.Name(), test.ShouldResemble, conf.ResourceName())
		test.That(t, g.Close(context.Background()), test.ShouldBeNil)
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
		_, err := newRTKMovementSensor(ctx, deps, conf, logger)
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

		_, err := newRTKMovementSensor(ctx, deps, conf, logger)
		test.That(t, err, test.ShouldNotBeNil)
	})
} */

func TestReadingsRTK(t *testing.T) {
	logger := logging.NewTestLogger(t)
	ctx := context.Background()
	cancelCtx, cancelFunc := context.WithCancel(ctx)
	g := RTKMovementSensor{
		cancelCtx:  cancelCtx,
		cancelFunc: cancelFunc,
		logger:     logger,
	}

	g.Nmeamovementsensor = &fake.MovementSensor{}

	status, err := g.NtripStatus()
	test.That(t, err, test.ShouldBeNil)
	test.That(t, status, test.ShouldEqual, false)

	loc1, alt1, err := g.Position(ctx, make(map[string]interface{}))
	test.That(t, err, test.ShouldBeNil)
	test.That(t, loc1, test.ShouldResemble, geo.NewPoint(40.7, -73.98))
	test.That(t, alt1, test.ShouldEqual, alt)

	speed1, err := g.LinearVelocity(ctx, make(map[string]interface{}))
	test.That(t, err, test.ShouldBeNil)
	test.That(t, speed1.Y, test.ShouldEqual, speed)

	fix1, err := g.ReadFix(ctx)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, fix1, test.ShouldEqual, fix)
}

func TestCloseRTK(t *testing.T) {
	logger := logging.NewTestLogger(t)
	ctx := context.Background()
	cancelCtx, cancelFunc := context.WithCancel(ctx)
	g := RTKMovementSensor{
		cancelCtx:   cancelCtx,
		cancelFunc:  cancelFunc,
		logger:      logger,
		ntripClient: &NtripInfo{},
	}
	g.Nmeamovementsensor = &fake.MovementSensor{}

	err := g.Close(ctx)
	test.That(t, err, test.ShouldBeNil)
}

// Helpers

// // mock ntripinfo client.
// func makeMockNtripClient() *NtripInfo {
// 	return &NtripInfo{}
// }
