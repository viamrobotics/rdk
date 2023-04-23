package gpsrtk

import (
	"context"
	"io"
	"strings"
	"testing"

	"github.com/edaniels/golog"
	"go.viam.com/test"
	"go.viam.com/utils"

	"go.viam.com/rdk/components/board"
	"go.viam.com/rdk/components/movementsensor"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/testutils/inject"
	rutils "go.viam.com/rdk/utils"
)

const (
	testBoardName = "board1"
	testBusName   = "i2c1"
)

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

	return deps
}

func TestValidate(t *testing.T) {
	fakecfg := &StationConfig{
		CorrectionSource: "",
		Children:         []string{},
		SurveyIn:         "",
		RequiredAccuracy: 0,
		RequiredTime:     0,
		SerialConfig:     &SerialConfig{},
		I2CConfig:        &I2CConfig{},
		NtripConfig:      &NtripConfig{},
	}
	path := "path"
	_, err := fakecfg.Validate(path)
	test.That(t, err, test.ShouldBeError,
		utils.NewConfigValidationFieldRequiredError(path, "correction_source"))

	fakecfg.CorrectionSource = "notvalid"
	_, err = fakecfg.Validate("path")
	test.That(t, err, test.ShouldBeError, ErrStationValidation)

	// ntrip
	fakecfg.CorrectionSource = "ntrip"
	_, err = fakecfg.Validate(path)
	test.That(t, err, test.ShouldBeError, utils.NewConfigValidationFieldRequiredError(path, "ntrip_addr"))

	fakecfg.NtripConfig.NtripAddr = "some-ntrip-address"
	fakecfg.NtripPath = "some-ntrip-path"
	_, err = fakecfg.Validate("path")
	test.That(t, err, test.ShouldBeNil)

	// serial
	fakecfg.CorrectionSource = "serial"
	_, err = fakecfg.Validate(path)
	test.That(t, err, test.ShouldBeError, utils.NewConfigValidationFieldRequiredError(path, "serial_correction_path"))

	fakecfg.SerialConfig.SerialCorrectionPath = "some-serial-path"
	_, err = fakecfg.Validate("path")
	test.That(t, err, test.ShouldBeNil)

	// I2C
	fakecfg.CorrectionSource = "i2c"
	_, err = fakecfg.Validate("path")
	test.That(t, err, test.ShouldBeError, utils.NewConfigValidationFieldRequiredError(path, "board"))
}

func TestRTK(t *testing.T) {
	logger := golog.NewTestLogger(t)
	ctx := context.Background()
	deps := setupDependencies(t)

	// test NTRIPConnection Source
	conf := resource.Config{
		Name:  "rtk1",
		Model: resource.DefaultModelFamily.WithModel("rtk-station"),
		API:   movementsensor.API,
		ConvertedAttributes: &StationConfig{
			CorrectionSource: "ntrip",
			Board:            testBoardName,
			NtripConfig: &NtripConfig{
				NtripAddr:            "some_ntrip_address",
				NtripConnectAttempts: 10,
				NtripMountpoint:      "NJI2",
				NtripPass:            "",
				NtripUser:            "",
			},
		},
	}

	// TODO(RSDK-2698): this test is not really doing anything since it needs a mocked
	// I2C; it used to just test a random error; it still does.
	g, err := newRTKStation(ctx, deps, conf, logger)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, g.Name(), test.ShouldResemble, conf.ResourceName())
	test.That(t, g.Close(context.Background()), test.ShouldBeNil)

	// test serial connection source
	path := "/dev/serial/by-id/usb-u-blox_AG_-_www.u-blox.com_u-blox_GNSS_receiver-if00"
	conf = resource.Config{
		Name:  "rtk1",
		Model: resource.DefaultModelFamily.WithModel("rtk-station"),
		API:   movementsensor.API,
		ConvertedAttributes: &StationConfig{
			CorrectionSource: "serial",
			SerialConfig: &SerialConfig{
				SerialCorrectionPath: path,
			},
		},
	}

	// TODO(RSDK-2698): this test is not really doing anything since it needs a mocked
	// I2C; it used to just test a random error; it still does.
	_, err = newRTKStation(ctx, deps, conf, logger)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "no such file or directory")

	// test I2C correction source
	conf = resource.Config{
		Name:       "rtk1",
		Model:      resource.DefaultModelFamily.WithModel("rtk-station"),
		API:        movementsensor.API,
		Attributes: rutils.AttributeMap{},
		ConvertedAttributes: &StationConfig{
			CorrectionSource: "i2c",
			Board:            testBoardName,
			SurveyIn:         "",
			I2CConfig: &I2CConfig{
				I2CBus: testBusName,
			},
			NtripConfig: &NtripConfig{
				NtripAddr:            "some_ntrip_address",
				NtripConnectAttempts: 10,
				NtripMountpoint:      "NJI2",
				NtripPass:            "",
				NtripUser:            "",
				NtripPath:            path,
			},
		},
	}

	// TODO(RSDK-2698): this test is not really doing anything since it needs a mocked
	// I2C; it used to just test a random error; it still does.
	g, err = newRTKStation(ctx, deps, conf, logger)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, g.Name(), test.ShouldResemble, conf.ResourceName())
	test.That(t, g.Close(context.Background()), test.ShouldBeNil)

	// test invalid source
	conf = resource.Config{
		Name:  "rtk1",
		Model: resource.DefaultModelFamily.WithModel("rtk-station"),
		API:   movementsensor.API,
		Attributes: rutils.AttributeMap{
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
		ConvertedAttributes: &StationConfig{
			CorrectionSource: "invalid",
		},
	}
	_, err = newRTKStation(ctx, deps, conf, logger)
	test.That(t, err, test.ShouldNotBeNil)
}

func TestClose(t *testing.T) {
	logger := golog.NewTestLogger(t)
	ctx := context.Background()
	cancelCtx, cancelFunc := context.WithCancel(ctx)
	g := rtkStation{cancelCtx: cancelCtx, cancelFunc: cancelFunc, logger: logger}
	r := io.NopCloser(strings.NewReader("hello world"))
	n := &ntripCorrectionSource{
		cancelCtx:        cancelCtx,
		cancelFunc:       cancelFunc,
		logger:           logger,
		correctionReader: r,
	}
	n.info = makeMockNtripClient()
	g.correction = n

	err := g.Close(ctx)
	test.That(t, err, test.ShouldBeNil)

	g = rtkStation{cancelCtx: cancelCtx, cancelFunc: cancelFunc, logger: logger}
	s := &serialCorrectionSource{
		cancelCtx:        cancelCtx,
		cancelFunc:       cancelFunc,
		logger:           logger,
		correctionReader: r,
	}
	g.correction = s

	err = g.Close(ctx)
	test.That(t, err, test.ShouldBeNil)

	g = rtkStation{cancelCtx: cancelCtx, cancelFunc: cancelFunc, logger: logger}
	i := &i2cCorrectionSource{
		cancelCtx:        cancelCtx,
		cancelFunc:       cancelFunc,
		logger:           logger,
		correctionReader: r,
	}
	g.correction = i

	err = g.Close(ctx)
	test.That(t, err, test.ShouldBeNil)
}

func TestRTKStationConnect(t *testing.T) {
	logger := golog.NewTestLogger(t)
	ctx := context.Background()
	cancelCtx, cancelFunc := context.WithCancel(ctx)
	info := &NtripInfo{
		URL:                "invalidurl",
		Username:           "user",
		Password:           "pwd",
		MountPoint:         "",
		MaxConnectAttempts: 10,
	}
	g := &ntripCorrectionSource{
		cancelCtx:  cancelCtx,
		cancelFunc: cancelFunc,
		logger:     logger,
		info:       info,
	}

	// create new ntrip client and connect
	err := g.Connect()
	test.That(t, err, test.ShouldNotBeNil)

	g.info.URL = "http://fakeurl"
	err = g.Connect()
	test.That(t, err, test.ShouldBeNil)

	err = g.GetStream()
	test.That(t, err, test.ShouldNotBeNil)

	g.info.MountPoint = "mp"
	err = g.GetStream()
	test.That(t, err.Error(), test.ShouldContainSubstring, "lookup fakeurl")
}
