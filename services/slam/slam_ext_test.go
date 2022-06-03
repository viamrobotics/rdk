// This file in the slam package validates the inputted config. Additional testing is done on exported
// functions in the slam_test package.
package slam

import (
	"io/ioutil"
	"os"
	"testing"

	"github.com/edaniels/golog"
	"github.com/pkg/errors"
	"go.viam.com/test"
)

func TestConfigValidation(t *testing.T) {
	logger := golog.NewTestLogger(t)

	name1, err := createTempFolderArchitecture(true)
	test.That(t, err, test.ShouldBeNil)

	cfg := &AttrConfig{
		Algorithm:        "cartographer",
		Sensors:          []string{"rplidar"},
		ConfigParams:     map[string]string{"mode": "2d"},
		DataDirectory:    name1,
		InputFilePattern: "100:300:5",
	}

	err = runtimeConfigValidation(cfg, logger)
	test.That(t, err, test.ShouldBeNil)

	t.Run("run test of config with no sensor", func(t *testing.T) {
		cfg.Sensors = []string{}
		err = runtimeConfigValidation(cfg, logger)
		test.That(t, err, test.ShouldBeNil)
	})

	t.Run("SLAM config slam mode test with invalid mode for library", func(t *testing.T) {
		cfg.ConfigParams["mode"] = ""
		err = runtimeConfigValidation(cfg, logger)
		test.That(t, err, test.ShouldBeError,
			errors.Errorf("getting data with specified algorithm %v, and desired mode %v", cfg.Algorithm, cfg.ConfigParams["mode"]))
	})

	t.Run("SLAM config input file pattern tests with bad pattern", func(t *testing.T) {
		cfg.ConfigParams["mode"] = "2d"
		cfg.InputFilePattern = "dd:300:3"
		err = runtimeConfigValidation(cfg, logger)
		test.That(t, err, test.ShouldBeError,
			errors.Errorf("input_file_pattern (%v) does not match the regex pattern (\\d+):(\\d+):(\\d+)", cfg.InputFilePattern))
	})

	t.Run("SLAM config input file pattern tests with initial file larger then final fail", func(t *testing.T) {
		cfg.InputFilePattern = "500:300:3"
		err = runtimeConfigValidation(cfg, logger)
		test.That(t, err, test.ShouldBeError,
			errors.Errorf("second value in input file pattern must be larger than the first [%v]", cfg.InputFilePattern))
	})

	t.Run("SLAM config input file pattern tests with 0 interval", func(t *testing.T) {
		cfg.InputFilePattern = "1:15:0"
		err = runtimeConfigValidation(cfg, logger)
		test.That(t, err, test.ShouldBeError,
			errors.New("the file input pattern's interval must be greater than zero"))
	})

	t.Run("SLAM config check on specified algorithm", func(t *testing.T) {
		cfg.Algorithm = "wrong_algo"
		err = runtimeConfigValidation(cfg, logger)
		test.That(t, err, test.ShouldBeError, errors.Errorf("%v algorithm specified not in implemented list", cfg.Algorithm))
	})
}

func createTempFolderArchitecture(validArch bool) (string, error) {
	name, err := ioutil.TempDir("", "*")
	if err != nil {
		return "nil", err
	}

	if validArch {
		if err := os.Mkdir(name+"/map", os.ModePerm); err != nil {
			return "", err
		}
		if err := os.Mkdir(name+"/data", os.ModePerm); err != nil {
			return "", err
		}
		if err := os.Mkdir(name+"/config", os.ModePerm); err != nil {
			return "", err
		}
	}
	return name, nil
}
