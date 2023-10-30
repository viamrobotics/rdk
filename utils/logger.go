package utils

import (
	"go.uber.org/zap/zapcore"
	"go.viam.com/rdk/logging"
)

// NewFilePathDebugLogger is intended as a debug only tool & should not be used in prod
// logs using debug configuration to log to both stderr, stdout & a filepath.
func NewFilePathDebugLogger(filepath, name string) (logging.Logger, error) {
	logConfig := logging.NewZapLoggerConfig()
	logConfig.OutputPaths = append(logConfig.OutputPaths, filepath)
	logConfig.ErrorOutputPaths = append(logConfig.ErrorOutputPaths, filepath)
	logConfig.Level.SetLevel(zapcore.DebugLevel)
	logger, err := logConfig.Build()
	if err != nil {
		return nil, err
	}

	return logging.FromZapCompatible(logger.Sugar().Named(name)), nil
}
