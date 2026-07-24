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
				"rdk.resource_manager":            "WARN",
				"rdk.resource_manager.modmanager": "INFO",
				"rdk.network_traffic":             "INFO",
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
				"rdk.resource_manager.modmanager":   "ERROR",
				"rdk.test_manager.modmanager":       "ERROR",
				"rdk.resource_manager.test_manager": "INFO",
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

		testRegistry.Update(tc.loggerConfig, NewLogger("error-logger"))
		test.That(t, verifySetLevels(testRegistry, tc.expectedMatches), test.ShouldBeTrue)
	}
}
