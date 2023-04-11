package config

import (
	"testing"

	"github.com/edaniels/golog"
	"go.viam.com/test"
	"go.viam.com/utils"

	"go.viam.com/rdk/config"
	"go.viam.com/rdk/resource"
)

var (
	_true  = true
	_false = false
)

func TestDetermineDeleteProcessedData(t *testing.T) {
	logger := golog.NewTestLogger(t)

	t.Run("No delete_processed_data provided", func(t *testing.T) {
		deleteProcessedData := DetermineDeleteProcessedData(logger, nil, false)
		test.That(t, deleteProcessedData, test.ShouldBeFalse)

		deleteProcessedData = DetermineDeleteProcessedData(logger, nil, true)
		test.That(t, deleteProcessedData, test.ShouldBeTrue)
	})

	t.Run("False delete_processed_data", func(t *testing.T) {
		deleteProcessedData := DetermineDeleteProcessedData(logger, &_false, false)
		test.That(t, deleteProcessedData, test.ShouldBeFalse)

		deleteProcessedData = DetermineDeleteProcessedData(logger, &_false, true)
		test.That(t, deleteProcessedData, test.ShouldBeFalse)
	})

	t.Run("True delete_processed_data", func(t *testing.T) {
		deleteProcessedData := DetermineDeleteProcessedData(logger, &_true, false)
		test.That(t, deleteProcessedData, test.ShouldBeFalse)

		deleteProcessedData = DetermineDeleteProcessedData(logger, &_true, true)
		test.That(t, deleteProcessedData, test.ShouldBeTrue)
	})
}

func TestDetermineUseLiveData(t *testing.T) {
	logger := golog.NewTestLogger(t)
	t.Run("No use_live_data specified", func(t *testing.T) {
		useLiveData, err := DetermineUseLiveData(logger, nil, []string{})
		test.That(t, err, test.ShouldBeError, newError("use_live_data is a required input parameter"))
		test.That(t, useLiveData, test.ShouldBeFalse)

		useLiveData, err = DetermineUseLiveData(logger, nil, []string{"camera"})
		test.That(t, err, test.ShouldBeError, newError("use_live_data is a required input parameter"))
		test.That(t, useLiveData, test.ShouldBeFalse)
	})
	t.Run("False use_live_data", func(t *testing.T) {
		useLiveData, err := DetermineUseLiveData(logger, &_false, []string{})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, useLiveData, test.ShouldBeFalse)

		useLiveData, err = DetermineUseLiveData(logger, &_false, []string{"camera"})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, useLiveData, test.ShouldBeFalse)
	})
	t.Run("True use_live_data", func(t *testing.T) {
		useLiveData, err := DetermineUseLiveData(logger, &_true, []string{})
		test.That(t, err, test.ShouldBeError, newError("sensors field cannot be empty when use_live_data is set to true"))
		test.That(t, useLiveData, test.ShouldBeFalse)

		useLiveData, err = DetermineUseLiveData(logger, &_true, []string{"camera"})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, useLiveData, test.ShouldBeTrue)
	})
}

// makeCfgService creates the simplest possible config that can pass validation.
func makeCfgService() config.Service {
	model := resource.NewDefaultModel(resource.ModelName("test"))
	cfgService := config.Service{Name: "test", Type: "slam", Model: model}
	cfgService.Attributes = make(map[string]interface{})
	cfgService.Attributes["config_params"] = map[string]string{
		"mode": "test mode",
	}
	cfgService.Attributes["data_dir"] = "path"
	cfgService.Attributes["use_live_data"] = true
	return cfgService
}

func TestNewAttrConf(t *testing.T) {
	testCfgPath := "services.slam.attributes.fake"
	logger := golog.NewTestLogger(t)

	t.Run("Empty config", func(t *testing.T) {
		model := resource.NewDefaultModel(resource.ModelName("test"))
		cfgService := config.Service{Name: "test", Type: "slam", Model: model}
		_, err := NewAttrConfig(cfgService)
		test.That(t, err, test.ShouldBeError)
	})

	t.Run("Simplest valid config", func(t *testing.T) {
		cfgService := makeCfgService()
		_, err := NewAttrConfig(cfgService)
		test.That(t, err, test.ShouldBeNil)
	})

	t.Run("Config without required fields", func(t *testing.T) {
		// Test for missing main attribute fields
		requiredFields := []string{"data_dir", "use_live_data"}
		for _, requiredField := range requiredFields {
			logger.Debugf("Testing SLAM config without %s", requiredField)
			cfgService := makeCfgService()
			delete(cfgService.Attributes, requiredField)
			_, err := NewAttrConfig(cfgService)
			test.That(t, err, test.ShouldBeError, newError(utils.NewConfigValidationFieldRequiredError(testCfgPath, requiredField).Error()))
		}
		// Test for missing config_params attributes
		logger.Debug("Testing SLAM config without config_params[mode]")
		cfgService := makeCfgService()
		delete(cfgService.Attributes["config_params"].(map[string]string), "mode")
		_, err := NewAttrConfig(cfgService)
		test.That(t, err, test.ShouldBeError, newError(utils.NewConfigValidationFieldRequiredError(testCfgPath, "config_params[mode]").Error()))
	})

	t.Run("Config with invalid parameter type", func(t *testing.T) {
		cfgService := makeCfgService()
		cfgService.Attributes["use_live_data"] = "true"
		_, err := NewAttrConfig(cfgService)
		test.That(t, err, test.ShouldBeError)
		cfgService.Attributes["use_live_data"] = true
		_, err = NewAttrConfig(cfgService)
		test.That(t, err, test.ShouldBeNil)
	})

	t.Run("Config with out of range values", func(t *testing.T) {
		cfgService := makeCfgService()
		cfgService.Attributes["data_rate_msec"] = -1
		_, err := NewAttrConfig(cfgService)
		test.That(t, err, test.ShouldBeError)
		cfgService.Attributes["data_rate_msec"] = 1
		cfgService.Attributes["map_rate_sec"] = -1
		_, err = NewAttrConfig(cfgService)
		test.That(t, err, test.ShouldBeError)
	})

	t.Run("All parameters e2e", func(t *testing.T) {
		cfgService := makeCfgService()
		cfgService.Attributes["sensors"] = []string{"a", "b"}
		cfgService.Attributes["data_rate_msec"] = 1001
		cfgService.Attributes["map_rate_sec"] = 1002
		cfgService.Attributes["port"] = "47"
		cfgService.Attributes["delete_processed_data"] = true

		cfgService.Attributes["config_params"] = map[string]string{
			"mode":    "test mode",
			"value":   "0",
			"value_2": "test",
		}
		cfg, err := NewAttrConfig(cfgService)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, cfg.DataDirectory, test.ShouldEqual, cfgService.Attributes["data_dir"])
		test.That(t, *cfg.UseLiveData, test.ShouldEqual, cfgService.Attributes["use_live_data"])
		test.That(t, cfg.Sensors, test.ShouldResemble, cfgService.Attributes["sensors"])
		test.That(t, cfg.DataRateMsec, test.ShouldEqual, cfgService.Attributes["data_rate_msec"])
		test.That(t, *cfg.MapRateSec, test.ShouldEqual, cfgService.Attributes["map_rate_sec"])
		test.That(t, cfg.Port, test.ShouldEqual, cfgService.Attributes["port"])
		test.That(t, cfg.ConfigParams, test.ShouldResemble, cfgService.Attributes["config_params"])
	})
}

func TestGetOptionalParameters(t *testing.T) {
	logger := golog.NewTestLogger(t)

	t.Run("Pass default parameters", func(t *testing.T) {
		cfgService := makeCfgService()
		cfgService.Attributes["sensors"] = []string{"a"}
		cfgService.Attributes["use_live_data"] = true
		cfg, err := NewAttrConfig(cfgService)
		test.That(t, err, test.ShouldBeNil)
		port, dataRateMsec, mapRateSec, useLiveData, deleteProcessedData, err := GetOptionalParameters(cfg, "localhost", 1001, 1002, logger)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, port, test.ShouldResemble, "localhost")
		test.That(t, dataRateMsec, test.ShouldEqual, 1001)
		test.That(t, mapRateSec, test.ShouldEqual, 1002)
		test.That(t, useLiveData, test.ShouldEqual, true)
		test.That(t, deleteProcessedData, test.ShouldEqual, true)
	})

	t.Run("Live data without sensors", func(t *testing.T) {
		cfgService := makeCfgService()
		cfgService.Attributes["use_live_data"] = true
		cfg, err := NewAttrConfig(cfgService)
		test.That(t, err, test.ShouldBeNil)
		_, _, _, _, _, err = GetOptionalParameters(cfg, "localhost", 1001, 1002, logger)
		test.That(t, err, test.ShouldBeError, newError("sensors field cannot be empty when use_live_data is set to true"))
	})
}
