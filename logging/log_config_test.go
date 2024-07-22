package logging

import (
	"strings"
	"testing"

	"go.viam.com/test"
)

func verifySetLevels(registry *loggerRegistry, expectedMatches map[string]string) bool {
	for name, level := range expectedMatches {
		logger, ok := registry.loggerNamed(name)
		if !ok || !strings.EqualFold(level, logger.GetLevel().String()) {
			return false
		}
	}
	return true
}

func createTestRegistry(loggerNames []string) *loggerRegistry {
	manager := newLoggerManager()
	for _, name := range loggerNames {
		manager.registerLogger(name, NewLogger(name))
	}
	return manager
}

func TestValidatePattern(t *testing.T) {
	// logger pattern matching
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

	// resource pattern matching
	test.That(t, validatePattern("rdk.rdk:service:encoder/encoder1"), test.ShouldBeTrue)
	test.That(t, validatePattern("rdk.rdk:component:motor/motor1"), test.ShouldBeTrue)
	test.That(t, validatePattern("rdk.acme:*:motor/motor1"), test.ShouldBeTrue)
	test.That(t, validatePattern("rdk.rdk:service:navigation/test-navigation"), test.ShouldBeTrue)
	test.That(t, validatePattern("rdk.*:*:motor/*"), test.ShouldBeTrue)
	test.That(t, validatePattern("rdk.rdk:remote:/foo"), test.ShouldBeTrue)

	test.That(t, validatePattern("fake.rdk:service:encoder/encoder1"), test.ShouldBeFalse)
	test.That(t, validatePattern("rdk.rdk:service:encoder/encoder1 1"), test.ShouldBeFalse)
	test.That(t, validatePattern("1 rdk.rdk:service:encoder/encoder1"), test.ShouldBeFalse)
	test.That(t, validatePattern("rdk.rdk:fake:encoder/encoder1"), test.ShouldBeFalse)
	test.That(t, validatePattern("rdk.:service:encoder/encoder1"), test.ShouldBeFalse)
	test.That(t, validatePattern("rdk.rdk:service:/encoder"), test.ShouldBeFalse)
	test.That(t, validatePattern("rdk.rdk:service:encoder/"), test.ShouldBeFalse)
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
		{
			loggerConfig: []LoggerPatternConfig{
				{
					Pattern: "a.b",
					Level:   "DEBUG",
				},
			},
			loggerNames: []string{
				"a.b.c",
			},
			expectedMatches: map[string]string{
				"a.b.c": "INFO",
			},
			doesError: false,
		},
	}

	for _, tc := range tests {
		testRegistry := createTestRegistry(tc.loggerNames)

		err := testRegistry.updateLoggerRegistry(tc.loggerConfig)
		if tc.doesError {
			test.That(t, err, test.ShouldNotBeNil)
			continue
		}
		test.That(t, err, test.ShouldBeNil)

		test.That(t, verifySetLevels(testRegistry, tc.expectedMatches), test.ShouldBeTrue)
	}
}
