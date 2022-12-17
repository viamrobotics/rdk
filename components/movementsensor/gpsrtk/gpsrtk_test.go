package gpsrtk

import (
	"context"
	"testing"

	"github.com/edaniels/golog"
	geo "github.com/kellydunn/golang-geo"
	"go.viam.com/test"
	"go.viam.com/utils"

	"go.viam.com/rdk/components/board"
	"go.viam.com/rdk/components/movementsensor"
	"go.viam.com/rdk/components/movementsensor/fake"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/resource"
)

var (
	alt   = 50.5
	speed = 5.4
	fix   = 1
)

func TestValidateRTK(t *testing.T) {
	path := "path"
	fakecfg := &AttrConfig{NtripAttrConfig: &NtripAttrConfig{}}
	_, err := fakecfg.Validate(path)
	test.That(t, err, test.ShouldBeError,
		utils.NewConfigValidationFieldRequiredError(path, "correction_source"))

	fakecfg.CorrectionSource = "ntrip"
	_, err = fakecfg.Validate(path)
	test.That(t, err, test.ShouldBeError,
		utils.NewConfigValidationFieldRequiredError(path, "ntrip_addr"))

	fakecfg.NtripAttrConfig.NtripAddr = "http://fakeurl"
	_, err = fakecfg.Validate(path)
	test.That(
		t,
		err,
		test.ShouldBeError,
		utils.NewConfigValidationFieldRequiredError(path, "ntrip_path"),
	)

	fakecfg.NtripAttrConfig.NtripPath = "some-ntrip-path"
	_, err = fakecfg.Validate("path")
	test.That(t, err, test.ShouldBeNil)
}

func TestConnect(t *testing.T) {
	logger := golog.NewTestLogger(t)
	ctx := context.Background()
	cancelCtx, cancelFunc := context.WithCancel(ctx)
	g := RTKMovementSensor{
		cancelCtx:  cancelCtx,
		cancelFunc: cancelFunc,
		logger:     logger,
	}

	url := "http://fakeurl"
	username := "user"
	password := "pwd"
	mountPoint := "mp"

	// create new ntrip client and connect
	err := g.Connect("invalidurl", username, password, 10)
	g.ntripClient = makeMockNtripClient()

	test.That(t, err, test.ShouldNotBeNil)

	err = g.Connect(url, username, password, 10)
	test.That(t, err, test.ShouldBeNil)

	err = g.GetStream("", 10)
	test.That(t, err, test.ShouldNotBeNil)

	err = g.GetStream(mountPoint, 10)
	test.That(t, err.Error(), test.ShouldContainSubstring, "lookup fakeurl")
}

func TestNewRTKMovementSensor(t *testing.T) {
	path := "somepath"
	deps := setupDependencies(t)
	logger := golog.NewTestLogger(t)
	ctx := context.Background()

	t.Run("serial protocol", func(t *testing.T) {
		// serial protocol
		cfig := config.Component{
			Name:  "movementsensor1",
			Model: resource.NewDefaultModel("rtk"),
			Type:  movementsensor.SubtypeName,
			Attributes: config.AttributeMap{
				"ntrip_send_nmea":           true,
				"ntrip_connect_attempts":    10,
				"correction_input_protocol": "serial",
				"path":                      path,
			},
			ConvertedAttributes: &AttrConfig{
				CorrectionSource: "serial",
				Board:            "",
				SerialAttrConfig: &SerialAttrConfig{
					SerialPath:               path,
					SerialBaudRate:           0,
					SerialCorrectionPath:     path,
					SerialCorrectionBaudRate: 0,
				},
				NtripAttrConfig: &NtripAttrConfig{
					NtripAddr:            "some_ntrip_address",
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

		g, err := newRTKMovementSensor(ctx, deps, cfig, logger)
		passErr := "open " + path + ": no such file or directory"
		if err == nil || err.Error() != passErr {
			test.That(t, err, test.ShouldBeNil)
			test.That(t, g, test.ShouldNotBeNil)
		}
	})

	t.Run("I2C protocol", func(t *testing.T) {
		cfig := config.Component{
			Name:  "movementsensor1",
			Model: resource.NewDefaultModel("rtk"),
			Type:  movementsensor.SubtypeName,
			Attributes: config.AttributeMap{
				"ntrip_addr":                "some_ntrip_address",
				"i2c_addr":                  "",
				"ntrip_username":            "",
				"ntrip_password":            "",
				"ntrip_mountpoint":          "",
				"ntrip_path":                "",
				"ntrip_baud":                115200,
				"ntrip_send_nmea":           true,
				"ntrip_connect_attempts":    10,
				"correction_input_protocol": "I2C",
				"path":                      path,
				"board":                     testBoardName,
				"bus":                       testBusName,
			},
			ConvertedAttributes: &AttrConfig{
				CorrectionSource: "I2C",
				Board:            testBoardName,
				I2CAttrConfig: &I2CAttrConfig{
					I2CBus:      testBusName,
					I2cAddr:     0,
					I2CBaudRate: 115200,
				},
			},
		}

		g, err := newRTKMovementSensor(ctx, deps, cfig, logger)
		passErr := "board " + testBoardName + " is not local"

		if err == nil || err.Error() != passErr {
			test.That(t, err, test.ShouldBeNil)
			test.That(t, g, test.ShouldNotBeNil)
		}
	})

	t.Run("invalid protocol", func(t *testing.T) {
		// invalid protocol
		cfig := config.Component{
			Name:  "movementsensor1",
			Model: resource.NewDefaultModel("rtk"),
			Type:  movementsensor.SubtypeName,
			Attributes: config.AttributeMap{
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
			ConvertedAttributes: &AttrConfig{},
		}
		_, err := newRTKMovementSensor(ctx, deps, cfig, logger)
		test.That(t, err, test.ShouldNotBeNil)
	})

	t.Run("no ntrip address", func(t *testing.T) {
		cfig := config.Component{
			Name:  "movementsensor1",
			Model: resource.NewDefaultModel("rtk"),
			Type:  movementsensor.SubtypeName,
			Attributes: config.AttributeMap{
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
			ConvertedAttributes: &AttrConfig{},
		}

		_, err := newRTKMovementSensor(ctx, deps, cfig, logger)
		test.That(t, err, test.ShouldNotBeNil)
	})
}

func TestReadingsRTK(t *testing.T) {
	logger := golog.NewTestLogger(t)
	ctx := context.Background()
	cancelCtx, cancelFunc := context.WithCancel(ctx)
	g := RTKMovementSensor{
		cancelCtx:  cancelCtx,
		cancelFunc: cancelFunc,
		logger:     logger,
	}

	g.nmeamovementsensor = &fake.MovementSensor{
		CancelCtx: cancelCtx,
		Logger:    logger,
	}

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
	logger := golog.NewTestLogger(t)
	ctx := context.Background()
	cancelCtx, cancelFunc := context.WithCancel(ctx)
	g := RTKMovementSensor{
		cancelCtx:   cancelCtx,
		cancelFunc:  cancelFunc,
		logger:      logger,
		ntripClient: &NtripInfo{},
	}
	g.nmeamovementsensor = &fake.MovementSensor{}

	err := g.Close()
	test.That(t, err, test.ShouldBeNil)
}

// Helpers

// mock ntripinfo client.
func makeMockNtripClient() *NtripInfo {
	return &NtripInfo{}
}

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
