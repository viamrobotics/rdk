package config

import (
	"testing"

	"go.uber.org/zap"
	"go.viam.com/test"

	"go.viam.com/rdk/logging"
)

func TestConfigDebugFlag(t *testing.T) {
	logConfig := logging.NewZapLoggerConfig()
	globalLogger := logging.FromZapCompatible(zap.Must(logConfig.Build()).Sugar())
	levelChangeLogger, logs := logging.NewObservedTestLogger(t)

	for _, cmdLineValue := range []bool{true, false} {
		for _, fileDebugValue := range []bool{true, false} {
			for _, cloudDebugValue := range []bool{true, false} {
				logs.TakeAll()

				InitLoggingSettings(globalLogger, levelChangeLogger, cmdLineValue)
				test.That(t, logs.FilterMessageSnippet("Log level initialized:").Len(), test.ShouldEqual, 1)

				UpdateFileConfigDebug(fileDebugValue)
				UpdateCloudConfigDebug(cloudDebugValue)

				expectedDebugEnabled := cmdLineValue || fileDebugValue || cloudDebugValue
				if expectedDebugEnabled {
					test.That(t, globalLogger.Level().Enabled(zap.DebugLevel), test.ShouldBeTrue)
				} else {
					// Debug must turn back off when no flag requests it (RSDK-13456).
					test.That(t, globalLogger.Level().Enabled(zap.DebugLevel), test.ShouldBeFalse)
				}
				test.That(t, globalLogger.Level().Enabled(zap.InfoLevel), test.ShouldBeTrue)
			}
		}
	}
}

func TestLogPatternSetsModuleLogLevel(t *testing.T) {
	levelChangeLogger := logging.NewTestLogger(t)
	logger, registry := logging.NewLoggerWithRegistry("rdk")

	newCfg := func() *Config {
		return &Config{
			Modules: []Module{
				{Name: "matched"},
				{Name: "unmatched"},
				{Name: "preset", LogLevel: "info"},
			},
			LogConfig: []logging.LoggerPatternConfig{
				{Pattern: "matched", Level: "debug"},
				{Pattern: "preset", Level: "debug"},
			},
		}
	}

	// With the global logger already at debug, module configs are left untouched.
	globalLogger := logging.NewLogger("global")
	InitLoggingSettings(globalLogger, levelChangeLogger, true)
	cfg := newCfg()
	UpdateLoggerRegistryFromConfig(registry, cfg, logger)
	test.That(t, cfg.Modules[0].LogLevel, test.ShouldEqual, "")
	test.That(t, cfg.Modules[1].LogLevel, test.ShouldEqual, "")
	test.That(t, cfg.Modules[2].LogLevel, test.ShouldEqual, "info")

	// With a non-debug global logger, a "debug" log pattern matching a module name
	// sets that module's LogLevel, forcing a restart with --log-level=debug.
	// An explicit LogLevel is never overwritten.
	InitLoggingSettings(globalLogger, levelChangeLogger, false)
	cfg = newCfg()
	UpdateLoggerRegistryFromConfig(registry, cfg, logger)
	test.That(t, cfg.Modules[0].LogLevel, test.ShouldEqual, "debug")
	test.That(t, cfg.Modules[1].LogLevel, test.ShouldEqual, "")
	test.That(t, cfg.Modules[2].LogLevel, test.ShouldEqual, "info")
}
