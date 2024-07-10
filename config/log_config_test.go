package config

import (
	"strings"
	"testing"

	"go.viam.com/test"

	"go.viam.com/rdk/logging"
)

func verifySetLevels(registry map[string]logging.Logger, expectedMatches map[string]string) bool {
	for name, level := range expectedMatches {
		logger, ok := registry[name]
		if !ok || !strings.EqualFold(level, logger.GetLevel().String()) {
			return false
		}
	}
	return true
}

func createTestRegistry(loggerNames ...string) map[string]logging.Logger {
	registry := make(map[string]logging.Logger)
	for _, name := range loggerNames {
		registry[name] = logging.NewLogger(name)
	}
	return registry
}

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

	test.That(t, validatePattern("_.robot_server.resource_manager"), test.ShouldBeFalse)
	test.That(t, validatePattern("-.robot_server"), test.ShouldBeFalse)
	test.That(t, validatePattern("robot_server.-"), test.ShouldBeFalse)
	test.That(t, validatePattern("robot_server.-"), test.ShouldBeFalse)
	test.That(t, validatePattern("robot_server.-.resource_manager"), test.ShouldBeFalse)
	test.That(t, validatePattern("robot_server._.resource_manager"), test.ShouldBeFalse)
}

func TestUpdateLoggerRegistry(t *testing.T) {
	t.Run("basic case", func(t *testing.T) {
		testRegistry := createTestRegistry("rdk.resource_manager", "rdk.resource_manager.modmanager", "rdk.network_traffic")
		logCfg := []LoggerPatternConfig{
			{
				Pattern: "rdk.resource_manager",
				Level:   "WARN",
			},
		}
		expectedMatches := map[string]string{
			"rdk.resource_manager": "WARN",
		}
		newRegistry, err := UpdateLoggerRegistry(logCfg, testRegistry)
		test.That(t, err, test.ShouldBeNil)

		test.That(t, verifySetLevels(newRegistry, expectedMatches), test.ShouldBeTrue)
	})

	t.Run("ending wildcard case", func(t *testing.T) {
		testRegistry := createTestRegistry("rdk.resource_manager", "rdk.test_manager.modmanager", "rdk.resource_manager.package.modmanager")
		logCfg := []LoggerPatternConfig{
			{
				Pattern: "rdk.*",
				Level:   "DEBUG",
			},
		}
		expectedMatches := map[string]string{
			"rdk.resource_manager":                    "DEBUG",
			"rdk.test_manager.modmanager":             "DEBUG",
			"rdk.resource_manager.package.modmanager": "DEBUG",
		}
		newRegistry, err := UpdateLoggerRegistry(logCfg, testRegistry)
		test.That(t, err, test.ShouldBeNil)

		test.That(t, verifySetLevels(newRegistry, expectedMatches), test.ShouldBeTrue)
	})

	t.Run("middle wildcard case", func(t *testing.T) {
		testRegistry := createTestRegistry("rdk.resource_manager.modmanager", "rdk.test_manager.modmanager", "rdk.resource_manager.test_manager")
		logCfg := []LoggerPatternConfig{
			{
				Pattern: "rdk.*.modmanager",
				Level:   "ERROR",
			},
		}
		expectedMatches := map[string]string{
			"rdk.resource_manager.modmanager": "ERROR",
			"rdk.test_manager.modmanager":     "ERROR",
		}
		newRegistry, err := UpdateLoggerRegistry(logCfg, testRegistry)
		test.That(t, err, test.ShouldBeNil)

		test.That(t, verifySetLevels(newRegistry, expectedMatches), test.ShouldBeTrue)
	})

	t.Run("overwrite existing registry entries", func(t *testing.T) {
		testRegistry := createTestRegistry("rdk.resource_manager")
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
		expectedMatches := map[string]string{
			"rdk.resource_manager": "WARN",
		}
		newRegistry, err := UpdateLoggerRegistry(logCfg, testRegistry)
		test.That(t, err, test.ShouldBeNil)

		test.That(t, verifySetLevels(newRegistry, expectedMatches), test.ShouldBeTrue)
	})

	t.Run("greedy wildcard matching case", func(t *testing.T) {
		testRegistry := createTestRegistry("rdk.resource_manager.modmanager", "rdk.resource_manager.package_manager.modmanager")
		logCfg := []LoggerPatternConfig{
			{
				Pattern: "rdk.*.modmanager",
				Level:   "WARN",
			},
		}
		expectedMatches := map[string]string{
			"rdk.resource_manager.modmanager":                 "WARN",
			"rdk.resource_manager.package_manager.modmanager": "WARN",
		}
		newRegistry, err := UpdateLoggerRegistry(logCfg, testRegistry)
		test.That(t, err, test.ShouldBeNil)

		test.That(t, verifySetLevels(newRegistry, expectedMatches), test.ShouldBeTrue)
	})

	t.Run("error case", func(t *testing.T) {
		testRegistry := createTestRegistry("rdk.resource_manager")
		logCfg := []LoggerPatternConfig{
			{
				Pattern: "_.*.modmanager",
				Level:   "DEBUG",
			},
		}
		_, err := UpdateLoggerRegistry(logCfg, testRegistry)
		test.That(t, err, test.ShouldNotBeNil)
	})
}
