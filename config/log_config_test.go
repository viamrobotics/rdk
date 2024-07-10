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

func createTestRegistry(loggerNames []string) map[string]logging.Logger {
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
	type testCfg struct {
		loggerConfig    []LoggerPatternConfig
		loggerNames     []string
		expectedMatches map[string]string
		doesError       bool
	}

	tests := []testCfg{
		{
			loggerConfig: []LoggerPatternConfig{
				{
					Pattern: "rdk.resource_manager",
					Level:   "WARN",
				},
			},
			loggerNames: []string{
				"rdk.resource_manager",
				"rdk.resource_manager.modmanager",
				"rdk.network_traffic",
			},
			expectedMatches: map[string]string{
				"rdk.resource_manager": "WARN",
			},
			doesError: false,
		},
		{
			loggerConfig: []LoggerPatternConfig{
				{
					Pattern: "rdk.*",
					Level:   "DEBUG",
				},
			},
			loggerNames: []string{
				"rdk.resource_manager",
				"rdk.test_manager.modmanager",
				"rdk.resource_manager.package.modmanager",
			},
			expectedMatches: map[string]string{
				"rdk.resource_manager":                    "DEBUG",
				"rdk.test_manager.modmanager":             "DEBUG",
				"rdk.resource_manager.package.modmanager": "DEBUG",
			},
			doesError: false,
		},
		{
			loggerConfig: []LoggerPatternConfig{
				{
					Pattern: "rdk.*.modmanager",
					Level:   "ERROR",
				},
			},
			loggerNames: []string{
				"rdk.resource_manager.modmanager",
				"rdk.test_manager.modmanager",
				"rdk.resource_manager.test_manager",
			},
			expectedMatches: map[string]string{
				"rdk.resource_manager.modmanager": "ERROR",
				"rdk.test_manager.modmanager":     "ERROR",
			},
			doesError: false,
		},
		{
			loggerConfig: []LoggerPatternConfig{
				{
					Pattern: "rdk.*",
					Level:   "DEBUG",
				},
				{
					Pattern: "rdk.resource_manager",
					Level:   "WARN",
				},
			},
			loggerNames: []string{
				"rdk.resource_manager",
			},
			expectedMatches: map[string]string{
				"rdk.resource_manager": "WARN",
			},
			doesError: false,
		},
		{
			loggerConfig: []LoggerPatternConfig{
				{
					Pattern: "rdk.*.modmanager",
					Level:   "WARN",
				},
			},
			loggerNames: []string{
				"rdk.resource_manager.modmanager",
				"rdk.resource_manager.package_manager.modmanager",
			},
			expectedMatches: map[string]string{
				"rdk.resource_manager.modmanager":                 "WARN",
				"rdk.resource_manager.package_manager.modmanager": "WARN",
			},
			doesError: false,
		},
		{
			loggerConfig: []LoggerPatternConfig{
				{
					Pattern: "_.*.modmanager",
					Level:   "DEBUG",
				},
			},
			loggerNames: []string{
				"rdk.resource_manager",
			},
			expectedMatches: map[string]string{},
			doesError:       true,
		},
	}

	for _, tc := range tests {
		testRegistry := createTestRegistry(tc.loggerNames)

		newRegistry, err := UpdateLoggerRegistry(tc.loggerConfig, testRegistry)
		if tc.doesError {
			test.That(t, err, test.ShouldNotBeNil)
			continue
		}
		test.That(t, err, test.ShouldBeNil)

		test.That(t, verifySetLevels(newRegistry, tc.expectedMatches), test.ShouldBeTrue)
	}
}
