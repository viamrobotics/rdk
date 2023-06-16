package rtkstation

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
	"go.viam.com/rdk/components/movementsensor/gpsnmea"
	"go.viam.com/rdk/components/movementsensor/gpsrtk"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/testutils/inject"
)

const (
	testBoardName   = "board1"
	testBusName     = "i2c1"
	testi2cAddr     = 44
	testSerialPath  = "some-path"
	testStationName = "testStation"
)

var c = make(chan []byte, 1024)

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

	logger := golog.NewTestLogger(t)
	ctx := context.Background()

	serialNMEA, _ := gpsnmea.NewSerialGPSNMEA(ctx, conf.ResourceName(), serialnmeaConf, logger)

	conf.Name = "rtk-sensor2"
	i2cNMEA, _ := gpsnmea.NewPmtkI2CGPSNMEA(ctx, deps, conf.ResourceName(), i2cnmeaConf, logger)

	rtkSensor1 := &gpsrtk.RTKMovementSensor{
		Nmeamovementsensor: serialNMEA, InputProtocol: serialStr,
	}

	rtkSensor2 := &gpsrtk.RTKMovementSensor{
		Nmeamovementsensor: i2cNMEA, InputProtocol: i2cStr,
	}

	deps[movementsensor.Named("rtk-sensor1")] = rtkSensor1
	deps[movementsensor.Named("rtk-sensor2")] = rtkSensor2

	return deps
}

func TestValidate(t *testing.T) {
	path := "path"
	tests := []struct {
		name          string
		stationConfig *StationConfig
		expectedErr   error
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
					Board:   testBoardName,
					I2CBus:  testBusName,
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
					I2CBus:  testBusName,
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
			} else if tc.stationConfig.CorrectionSource == i2cStr {
				test.That(t, err, test.ShouldBeNil)
				test.That(t, deps, test.ShouldNotBeNil)
				test.That(t, deps[0], test.ShouldEqual, testBoardName)
			}
		})
	}
}

func TestNewRTKStation(t *testing.T) {
	logger := golog.NewTestLogger(t)
	ctx := context.Background()
	deps := setupDependencies(t)

	c := make(chan []byte, 1024)

	tests := []struct {
		name        string
		config      resource.Config
		expectedErr error
	}{
		{
			name: "A valid config with serial connection should result in no errors",
			config: resource.Config{
				Name:  testStationName,
				Model: stationModel,
				API:   movementsensor.API,
				ConvertedAttributes: &StationConfig{
					CorrectionSource: "serial",
					Children:         []string{"rtk-sensor1"},
					RequiredAccuracy: 4,
					RequiredTime:     200,
					SerialConfig: &SerialConfig{
						SerialCorrectionPath:     "testChan",
						SerialCorrectionBaudRate: 9600,
						TestChan:                 c,
					},
					I2CConfig: &I2CConfig{},
				},
			},
		},
		{
			name: "A valid config with i2c connection should result in no errors",
			config: resource.Config{
				Name:  testStationName,
				Model: stationModel,
				API:   movementsensor.API,
				ConvertedAttributes: &StationConfig{
					CorrectionSource: "i2c",
					Children:         []string{"rtk-sensor2"},
					RequiredAccuracy: 4,
					RequiredTime:     200,
					SerialConfig:     &SerialConfig{},
					I2CConfig: &I2CConfig{
						Board:   testBoardName,
						I2CBus:  testBusName,
						I2cAddr: testi2cAddr,
					},
				},
			},
		},
		{
			name: "A rtk base station can send corrections to multiple children",
			config: resource.Config{
				Name:  testStationName,
				Model: stationModel,
				API:   movementsensor.API,
				ConvertedAttributes: &StationConfig{
					CorrectionSource: "serial",
					Children:         []string{"rtk-sensor1", "rtk-sensor2"},
					RequiredAccuracy: 4,
					RequiredTime:     200,
					SerialConfig: &SerialConfig{
						SerialCorrectionPath: "some-path",
						TestChan:             c,
					},
					I2CConfig: &I2CConfig{},
				},
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			g, err := newRTKStation(ctx, deps, tc.config, logger)
			test.That(t, err, test.ShouldBeNil)
			test.That(t, g.Name(), test.ShouldResemble, tc.config.ResourceName())
		})
	}
}

func TestClose(t *testing.T) {
	logger := golog.NewTestLogger(t)
	ctx := context.Background()
	cancelCtx, cancelFunc := context.WithCancel(ctx)
	r := io.NopCloser(strings.NewReader("hello world"))

	tests := []struct {
		name        string
		baseStation *rtkStation
		expectedErr error
	}{
		{
			name: "Should close serial with no errors",
			baseStation: &rtkStation{
				cancelCtx: cancelCtx, cancelFunc: cancelFunc, logger: logger, correctionSource: &serialCorrectionSource{
					cancelCtx:        cancelCtx,
					cancelFunc:       cancelFunc,
					logger:           logger,
					correctionReader: r,
				},
			},
		},
		{
			name: "should close i2c with no errors",
			baseStation: &rtkStation{
				cancelCtx: cancelCtx, cancelFunc: cancelFunc, logger: logger, correctionSource: &i2cCorrectionSource{
					cancelCtx:        cancelCtx,
					cancelFunc:       cancelFunc,
					logger:           logger,
					correctionReader: r,
				},
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.baseStation.Close(ctx)
			test.That(t, err, test.ShouldBeNil)
		})
	}
}
