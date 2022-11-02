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
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/resource"
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
	fakecfg := &StationConfig{
		CorrectionSource: "",
		Children:         []string{},
		SurveyIn:         "",
		RequiredAccuracy: 0,
		RequiredTime:     0,
		SerialAttrConfig: &SerialAttrConfig{},
		I2CAttrConfig:    &I2CAttrConfig{},
		NtripAttrConfig:  &NtripAttrConfig{},
	}
	path := "path"
	_, err := fakecfg.Validate(path)
	test.That(t, err, test.ShouldBeError,
		utils.NewConfigValidationFieldRequiredError(path, "correction_source"))

	fakecfg.CorrectionSource = "notvalid"
	_, err = fakecfg.Validate("path")
	test.That(t, err, test.ShouldBeError, "only serial, I2C, and ntrip are supported correction sources")

	// ntrip
	fakecfg.CorrectionSource = "ntrip"
	_, err = fakecfg.Validate(path)
	test.That(t, err, test.ShouldBeError, utils.NewConfigValidationFieldRequiredError(path, "ntrip_addr"))

	fakecfg.NtripAttrConfig.NtripAddr = "some-ntrip-address"
	fakecfg.NtripPath = "some-ntrip-path"
	_, err = fakecfg.Validate("path")
	test.That(t, err, test.ShouldBeNil)

	// serial
	fakecfg.CorrectionSource = "serial"
	_, err = fakecfg.Validate(path)
	test.That(t, err, test.ShouldBeError, utils.NewConfigValidationFieldRequiredError(path, "serial_correction_path"))

	fakecfg.SerialAttrConfig.SerialCorrectionPath = "some-serial-path"
	_, err = fakecfg.Validate("path")
	test.That(t, err, test.ShouldBeNil)

	// I2C
	fakecfg.CorrectionSource = "I2C"
	_, err = fakecfg.Validate("path")
	test.That(t, err, test.ShouldBeError, utils.NewConfigValidationFieldRequiredError(path, "board"))
}

func TestRTK(t *testing.T) {
	logger := golog.NewTestLogger(t)
	ctx := context.Background()
	deps := setupDependencies(t)

	// test NTRIPConnection Source
	cfig := config.Component{
		Name:  "rtk1",
		Model: resource.NewDefaultModel("rtk-station"),
		Type:  "gps",
		ConvertedAttributes: &StationConfig{
			CorrectionSource: "ntrip",
			Board:            testBoardName,
			NtripAttrConfig: &NtripAttrConfig{
				NtripAddr:            "some_ntrip_address",
				NtripConnectAttempts: 10,
				NtripMountpoint:      "NJI2",
				NtripPass:            "",
				NtripUser:            "",
			},
		},
	}

	g, err := newRTKStation(ctx, deps, cfig, logger)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, g, test.ShouldNotBeNil)

	// test serial connection source
	path := "/dev/serial/by-id/usb-u-blox_AG_-_www.u-blox.com_u-blox_GNSS_receiver-if00"
	cfig = config.Component{
		Name:  "rtk1",
		Model: resource.NewDefaultModel("rtk-station"),
		Type:  "gps",
		ConvertedAttributes: &StationConfig{
			CorrectionSource: "serial",
			SerialAttrConfig: &SerialAttrConfig{
				SerialCorrectionPath: path,
			},
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
		Name:       "rtk1",
		Model:      resource.NewDefaultModel("rtk-station"),
		Type:       "gps",
		Attributes: config.AttributeMap{},
		ConvertedAttributes: &StationConfig{
			CorrectionSource: "I2C",
			Board:            testBoardName,
			SurveyIn:         "",
			I2CAttrConfig: &I2CAttrConfig{
				I2CBus: testBusName,
			},
			NtripAttrConfig: &NtripAttrConfig{
				NtripAddr:            "some_ntrip_address",
				NtripConnectAttempts: 10,
				NtripMountpoint:      "NJI2",
				NtripPass:            "",
				NtripUser:            "",
				NtripPath:            path,
			},
		},
	}

	g, err = newRTKStation(ctx, deps, cfig, logger)
	passErr = "board " + testBoardName + " is not local"

	if err == nil || err.Error() != passErr {
		test.That(t, err, test.ShouldBeNil)
		test.That(t, g, test.ShouldNotBeNil)
	}

	// test invalid source
	cfig = config.Component{
		Name:  "rtk1",
		Model: resource.NewDefaultModel("rtk-station"),
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
		ConvertedAttributes: &StationConfig{
			CorrectionSource: "invalid",
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
	r := io.NopCloser(strings.NewReader("hello world"))
	n := &ntripCorrectionSource{
		cancelCtx:        cancelCtx,
		cancelFunc:       cancelFunc,
		logger:           logger,
		correctionReader: r,
	}
	n.info = makeMockNtripClient()
	g.correction = n

	err := g.Close()
	test.That(t, err, test.ShouldBeNil)

	g = rtkStation{cancelCtx: cancelCtx, cancelFunc: cancelFunc, logger: logger}
	s := &serialCorrectionSource{
		cancelCtx:        cancelCtx,
		cancelFunc:       cancelFunc,
		logger:           logger,
		correctionReader: r,
	}
	g.correction = s

	err = g.Close()
	test.That(t, err, test.ShouldBeNil)

	g = rtkStation{cancelCtx: cancelCtx, cancelFunc: cancelFunc, logger: logger}
	i := &i2cCorrectionSource{
		cancelCtx:        cancelCtx,
		cancelFunc:       cancelFunc,
		logger:           logger,
		correctionReader: r,
	}
	g.correction = i

	err = g.Close()
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
