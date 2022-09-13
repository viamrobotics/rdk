// This file in the slam package validates the inputted config. Additional testing is done on exported
// functions in the slam_test package.
package slam

import (
	"os"
	"testing"

	"github.com/edaniels/golog"
	"github.com/pkg/errors"
	"go.viam.com/test"
)

func TestConfigValidation(t *testing.T) {
	logger := golog.NewTestLogger(t)

	name1, err := createTempFolderArchitecture()
	test.That(t, err, test.ShouldBeNil)

	t.Run("valid config with sensor", func(t *testing.T) {
		cfg := getValidConfig(name1)
		mode, err := runtimeConfigValidation(cfg, logger)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, mode, test.ShouldEqual, "2d")
	})

	t.Run("run test of config with no sensor", func(t *testing.T) {
		cfg := getValidConfig(name1)
		cfg.Sensors = []string{}
		_, err = runtimeConfigValidation(cfg, logger)
		test.That(t, err, test.ShouldBeNil)
	})

	t.Run("SLAM config slam mode test with invalid mode for library", func(t *testing.T) {
		cfg := getValidConfig(name1)
		cfg.ConfigParams["mode"] = ""
		_, err = runtimeConfigValidation(cfg, logger)
		test.That(t, err, test.ShouldBeError,
			errors.Errorf("getting data with specified algorithm %v, and desired mode %v", cfg.Algorithm, cfg.ConfigParams["mode"]))
	})

	t.Run("SLAM config input file pattern tests with bad pattern", func(t *testing.T) {
		cfg := getValidConfig(name1)
		cfg.ConfigParams["mode"] = "2d"
		cfg.InputFilePattern = "dd:300:3"
		_, err = runtimeConfigValidation(cfg, logger)
		test.That(t, err, test.ShouldBeError,
			errors.Errorf("input_file_pattern (%v) does not match the regex pattern (\\d+):(\\d+):(\\d+)", cfg.InputFilePattern))
	})

	t.Run("SLAM config input file pattern tests with initial file larger then final fail", func(t *testing.T) {
		cfg := getValidConfig(name1)
		cfg.InputFilePattern = "500:300:3"
		_, err = runtimeConfigValidation(cfg, logger)
		test.That(t, err, test.ShouldBeError,
			errors.Errorf("second value in input file pattern must be larger than the first [%v]", cfg.InputFilePattern))
	})

	t.Run("SLAM config input file pattern tests with 0 interval", func(t *testing.T) {
		cfg := getValidConfig(name1)
		cfg.InputFilePattern = "1:15:0"
		_, err = runtimeConfigValidation(cfg, logger)
		test.That(t, err, test.ShouldBeError,
			errors.New("the file input pattern's interval must be greater than zero"))
	})

	t.Run("SLAM config check on specified algorithm", func(t *testing.T) {
		cfg := getValidConfig(name1)
		cfg.Algorithm = "wrong_algo"
		_, err = runtimeConfigValidation(cfg, logger)
		test.That(t, err, test.ShouldBeError, errors.Errorf("%v algorithm specified not in implemented list", cfg.Algorithm))
	})

	t.Run("SLAM config check data_rate_ms", func(t *testing.T) {
		cfg := getValidConfig(name1)
		cfg.DataRateMs = 10
		_, err = runtimeConfigValidation(cfg, logger)
		test.That(t, err, test.ShouldBeError, errors.New("cannot specify data_rate_ms less than 200"))
	})
}

func createTempFolderArchitecture() (string, error) {
	name, err := os.MkdirTemp("", "*")
	if err != nil {
		return "nil", err
	}

	if err := os.Mkdir(name+"/map", os.ModePerm); err != nil {
		return "", err
	}
	if err := os.Mkdir(name+"/data", os.ModePerm); err != nil {
		return "", err
	}
	if err := os.Mkdir(name+"/config", os.ModePerm); err != nil {
		return "", err
	}
	return name, nil
}

func getValidConfig(dataDirectory string) *AttrConfig {
	return &AttrConfig{
		Algorithm:        "cartographer",
		Sensors:          []string{"rplidar"},
		ConfigParams:     map[string]string{"mode": "2d"},
		DataDirectory:    dataDirectory,
		InputFilePattern: "100:300:5",
	}
}
