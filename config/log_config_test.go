package config

import (
	"testing"

	"go.viam.com/test"

	"go.viam.com/rdk/logging"
)

func TestValidatePattern(t *testing.T) {
	test.That(t, validatePattern("robot_server.resource_manager"), test.ShouldBeTrue)
	test.That(t, validatePattern("robot_server.resource_manager.*"), test.ShouldBeTrue)
	test.That(t, validatePattern("robot_server.*.resource_manager"), test.ShouldBeTrue)
	test.That(t, validatePattern("robot_server.*.*"), test.ShouldBeTrue)
	test.That(t, validatePattern("*.resource_manager"), test.ShouldBeTrue)
	test.That(t, validatePattern("*"), test.ShouldBeTrue)

	test.That(t, validatePattern("robot_server..resource_manager"), test.ShouldBeFalse)
	test.That(t, validatePattern("robot_server.resource_manager."), test.ShouldBeFalse)
	test.That(t, validatePattern(".robot_server.resource_manager"), test.ShouldBeFalse)
	test.That(t, validatePattern("robot_server.resource_manager.**"), test.ShouldBeFalse)
	test.That(t, validatePattern("robot_server.**.resource_manager"), test.ShouldBeFalse)
}

func TestUpdateLoggerRegistry(t *testing.T) {
	testRegistry := map[string]logging.Logger{
		"rdk.resource_manager":            logging.NewLogger("rdk.resource_manager"),
		"rdk.resource_manager.modmanager": logging.NewLogger("rdk.resource_manager.modmanager"),
		"rdk.network_traffic":             logging.NewLogger("rdk.network_traffic"),
	}
	t.Run("basic case", func(t *testing.T) {
		logCfg := []LoggerPatternConfig{
			{
				Pattern: "rdk.resource_manager",
				Level:   "WARN",
			},
		}
		newRegistry, err := UpdateLoggerRegistry(logCfg, testRegistry)
		test.That(t, err, test.ShouldBeNil)

		logger, ok := newRegistry["rdk.resource_manager"]
		test.That(t, ok, test.ShouldBeTrue)
		test.That(t, logger.GetLevel(), test.ShouldEqual, logging.WARN)

		testRegistry["rdk.resource_manager"].SetLevel(logging.INFO)
	})

	t.Run("wildcard case", func(t *testing.T) {
		logCfg := []LoggerPatternConfig{
			{
				Pattern: "rdk.*",
				Level:   "DEBUG",
			},
		}
		newRegistry, err := UpdateLoggerRegistry(logCfg, testRegistry)
		test.That(t, err, test.ShouldBeNil)

		for name, logger := range newRegistry {
			test.That(t, logger.GetLevel(), test.ShouldEqual, logging.DEBUG)
			newRegistry[name].SetLevel(logging.INFO)
		}
	})

	t.Run("overwrite existing registry entries", func(t *testing.T) {
		logCfg := []LoggerPatternConfig{
			{
				Pattern: "rdk.*",
				Level:   "DEBUG",
			},
			{
				Pattern: "rdk.resource_manager",
				Level:   "WARN",
			},
		}
		newRegistry, err := UpdateLoggerRegistry(logCfg, testRegistry)
		test.That(t, err, test.ShouldBeNil)

		logger, ok := newRegistry["rdk.resource_manager"]
		test.That(t, ok, test.ShouldBeTrue)
		test.That(t, logger.GetLevel(), test.ShouldEqual, logging.WARN)
	})
}
