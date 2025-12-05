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
				}
				test.That(t, globalLogger.Level().Enabled(zap.InfoLevel), test.ShouldBeTrue)
			}
		}
	}
}
