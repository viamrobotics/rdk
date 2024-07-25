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
	manager := newLoggerRegistry()
	for _, name := range loggerNames {
		manager.registerLogger(name, NewLogger(name))
	}
	return manager
}

func TestValidatePattern(t *testing.T) {
	t.Parallel()

	type testCfg struct {
		pattern string
		isValid bool
	}

	tests := []testCfg{
		// Valid patterns
		{"robot_server.resource_manager", true},
		{"robot_server.resource_manager.*", true},
		{"robot_server.*.resource_manager", true},
		{"robot_server.*.*", true},
		{"*.resource_manager", true},
		{"*", true},

		// Invalid patterns
		{"robot_server..resource_manager", false},
		{"robot_server.resource_manager.", false},
		{".robot_server.resource_manager", false},
		{"robot_server.resource_manager.**", false},
		{"robot_server.**.resource_manager", false},

		// Invalid patterns with special characters
		{"_.robot_server.resource_manager", false},
		{"-.robot_server", false},
		{"robot_server.-", false},
		{"robot_server.-.resource_manager", false},
		{"robot_server._.resource_manager", false},

		// Resource pattern matching (valid patterns)
		{"rdk.rdk:service:encoder/encoder1", true},
		{"rdk.rdk:component:motor/motor1", true},
		{"rdk.acme:*:motor/motor1", true},
		{"rdk.rdk:service:navigation/test-navigation", true},
		{"rdk.*:*:motor/*", true},
		{"rdk.rdk:remote:/foo", true},

		// Resource pattern matching (invalid patterns)
		{"fake.rdk:service:encoder/encoder1", false},
		{"rdk.rdk:service:encoder/encoder1 1", false},
		{"1 rdk.rdk:service:encoder/encoder1", false},
		{"rdk.rdk:fake:encoder/encoder1", false},
		{"rdk.:service:encoder/encoder1", false},
		{"rdk.rdk:service:/encoder", false},
		{"rdk.rdk:service:encoder/", false},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.pattern, func(t *testing.T) {
			t.Parallel()
			test.That(t, validatePattern(tc.pattern), test.ShouldEqual, tc.isValid)
		})
	}
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
