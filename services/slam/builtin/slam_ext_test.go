package builtin_test

import (
	"testing"

	"github.com/edaniels/golog"
	"github.com/pkg/errors"
	"go.viam.com/test"

	"go.viam.com/rdk/services/slam/builtin"
	slamConfig "go.viam.com/slam/config"
	slamTesthelper "go.viam.com/slam/testhelper"
)

func TestConfigValidation(t *testing.T) {
	logger := golog.NewTestLogger(t)

	name1, err := slamTesthelper.CreateTempFolderArchitecture(logger)
	test.That(t, err, test.ShouldBeNil)

	t.Run("valid config with sensor", func(t *testing.T) {
		cfg := getValidConfig(name1)
		model := "cartographer"
		mode, err := builtin.RuntimeConfigValidation(cfg, model, logger)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, mode, test.ShouldEqual, "2d")
	})

	t.Run("run test of config with no sensor", func(t *testing.T) {
		cfg := getValidConfig(name1)
		model := "cartographer"
		cfg.Sensors = []string{}
		_, err = builtin.RuntimeConfigValidation(cfg, model, logger)
		test.That(t, err, test.ShouldBeNil)
	})

	t.Run("SLAM config slam mode test with invalid mode for library", func(t *testing.T) {
		cfg := getValidConfig(name1)
		model := "cartographer"
		cfg.ConfigParams["mode"] = ""
		_, err = builtin.RuntimeConfigValidation(cfg, model, logger)
		test.That(t, err, test.ShouldBeError,
			errors.Errorf("getting data with specified algorithm %v, and desired mode %v", model, cfg.ConfigParams["mode"]))
	})

	t.Run("SLAM config check on specified algorithm", func(t *testing.T) {
		cfg := getValidConfig(name1)
		model := "wrong_algo"
		_, err = builtin.RuntimeConfigValidation(cfg, model, logger)
		test.That(t, err, test.ShouldBeError, errors.Errorf("%v algorithm specified not in implemented list", model))
	})
}

func getValidConfig(dataDirectory string) *slamConfig.AttrConfig {
	return &slamConfig.AttrConfig{
		Sensors:       []string{"rplidar"},
		ConfigParams:  map[string]string{"mode": "2d"},
		DataDirectory: dataDirectory,
	}
}
