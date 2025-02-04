package logging

import (
	"strings"
	"testing"

	"go.viam.com/test"
)

func verifySetLevels(registry *Registry, expectedMatches map[string]string) bool {
	for name, level := range expectedMatches {
		logger, ok := registry.loggerNamed(name)
		if !ok || !strings.EqualFold(level, logger.GetLevel().String()) {
			return false
		}
	}
	return true
}

func createTestRegistry(loggerNames []string) *Registry {
	manager := newRegistry()
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
		{"rdk.resource_manager.rdk:service:encoder/encoder1", true},
		{"rdk.resource_manager.rdk:component:motor/motor1", true},
		{"rdk.resource_manager.acme:*:motor/motor1", true},
		{"rdk.resource_manager.rdk:service:navigation/test-navigation", true},
		{"rdk.resource_manager.*:*:motor/*", true},
		{"rdk.resource_manager.rdk:remote:/foo", true},

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
		},
	}

	for _, tc := range tests {
		testRegistry := createTestRegistry(tc.loggerNames)

		err := testRegistry.Update(tc.loggerConfig, NewLogger("error-logger"))
		test.That(t, err, test.ShouldBeNil)
		test.That(t, verifySetLevels(testRegistry, tc.expectedMatches), test.ShouldBeTrue)
	}
}
