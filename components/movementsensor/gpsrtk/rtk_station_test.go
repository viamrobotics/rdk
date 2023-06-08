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
	path := "path"
	testi2cAddr := 44
	tests := []struct {
		name          string
		stationConfig *StationConfig
		expectedErr   error
		protocol      string
	}{
		{
			name: "A valid config with serial connection should result in no errors",
			stationConfig: &StationConfig{
				CorrectionSource: "serial",
				Children:         []string{},
				RequiredAccuracy: 4,
				RequiredTime:     200,
				SerialConfig: &SerialConfig{
					SerialCorrectionPath:     "some-path",
					SerialCorrectionBaudRate: 9600,
				},
				I2CConfig: &I2CConfig{},
			},
		},
		{
			name: "A valid config with i2c connection should result in no errors",
			stationConfig: &StationConfig{
				CorrectionSource: "i2c",
				Children:         []string{},
				RequiredAccuracy: 4,
				RequiredTime:     200,
				SerialConfig:     &SerialConfig{},
				I2CConfig: &I2CConfig{
					Board:   "pi",
					I2CBus:  "some-bus",
					I2cAddr: testi2cAddr,
				},
			},
		},
		{
			name: "A config without a correction source should result in error",
			stationConfig: &StationConfig{
				CorrectionSource: "",
				Children:         []string{},
				RequiredAccuracy: 4,
				RequiredTime:     200,
				SerialConfig:     &SerialConfig{},
				I2CConfig:        &I2CConfig{},
			},
			expectedErr: utils.NewConfigValidationFieldRequiredError(path, "correction_source"),
		},
		{
			name: "a config with no RequiredAccuracy should result in error",
			stationConfig: &StationConfig{
				CorrectionSource: "i2c",
				Children:         []string{},
				RequiredAccuracy: 0,
				RequiredTime:     0,
				SerialConfig:     &SerialConfig{},
				I2CConfig:        &I2CConfig{},
			},
			expectedErr: utils.NewConfigValidationFieldRequiredError(path, "required_accuracy"),
		},
		{
			name: "a config with no RequiredTime should result in error",
			stationConfig: &StationConfig{
				CorrectionSource: "i2c",
				Children:         []string{},
				RequiredAccuracy: 5,
				RequiredTime:     0,
				SerialConfig:     &SerialConfig{},
				I2CConfig:        &I2CConfig{},
			},
			expectedErr: utils.NewConfigValidationFieldRequiredError(path, "required_time"),
		},
		{
			name: "The required accuracy can only be values 1-5",
			stationConfig: &StationConfig{
				CorrectionSource: "i2c",
				Children:         []string{},
				RequiredAccuracy: 6,
				RequiredTime:     200,
				SerialConfig:     &SerialConfig{},
				I2CConfig:        &I2CConfig{},
			},
			expectedErr: errRequiredAccuracy,
		},
		{
			name: "serial station without a serial correction path should result in error",
			stationConfig: &StationConfig{
				CorrectionSource: "serial",
				Children:         []string{},
				RequiredAccuracy: 5,
				RequiredTime:     200,
				SerialConfig:     &SerialConfig{},
				I2CConfig:        &I2CConfig{},
			},
			expectedErr: utils.NewConfigValidationFieldRequiredError(path, "serial_correction_path"),
		},
		{
			name: "i2c station without a board should result in error",
			stationConfig: &StationConfig{
				CorrectionSource: "i2c",
				Children:         []string{},
				RequiredAccuracy: 5,
				RequiredTime:     200,
				SerialConfig:     &SerialConfig{},
				I2CConfig: &I2CConfig{
					I2CBus:  "some-bus",
					I2cAddr: testi2cAddr,
				},
			},
			expectedErr: utils.NewConfigValidationFieldRequiredError(path, "board"),
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			deps, err := tc.stationConfig.Validate(path)

			if tc.expectedErr != nil {
				test.That(t, err, test.ShouldBeError, tc.expectedErr)
				test.That(t, len(deps), test.ShouldEqual, 0)
			} else {
				if tc.protocol == i2cStr {
					test.That(t, deps[0], test.ShouldEqual, "pi")
				}
			}

		})
	}
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
			I2CConfig: &I2CConfig{
				I2CBus: testBusName,
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
