package config

import (
	"testing"

	"github.com/edaniels/golog"
	"go.uber.org/zap"
	"go.viam.com/test"
)

func TestConfigDebugFlag(t *testing.T) {
	logConfig := golog.NewDevelopmentLoggerConfig()
	logger := zap.Must(logConfig.Build()).Sugar()

	for _, cmdLineValue := range []bool{true, false} {
		for _, fileDebugValue := range []bool{true, false} {
			for _, cloudDebugValue := range []bool{true, false} {
				InitLoggingSettings(logger, cmdLineValue, logConfig.Level)
				UpdateFileConfigDebug(fileDebugValue)
				UpdateCloudConfigDebug(cloudDebugValue)

				expectedDebugEnabled := cmdLineValue || fileDebugValue || cloudDebugValue
				if expectedDebugEnabled {
					test.That(t, logger.Level().Enabled(zap.DebugLevel), test.ShouldBeTrue)
				}
				test.That(t, logger.Level().Enabled(zap.InfoLevel), test.ShouldBeTrue)
			}
		}
	}
}
