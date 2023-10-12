package utils

import (
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.viam.com/rdk/logging"
)

// NewFilePathDebugLogger is intended as a debug only tool & should not be used in prod
// logs using debug configuration to log to both stderr, stdout & a filepath.
func NewFilePathDebugLogger(filepath, name string) (logging.Logger, error) {
	logger, err := zap.Config{
		Level:    zap.NewAtomicLevelAt(zap.DebugLevel),
		Encoding: "console",
		EncoderConfig: zapcore.EncoderConfig{
			TimeKey:        "ts",
			LevelKey:       "level",
			NameKey:        "logger",
			CallerKey:      "caller",
			FunctionKey:    zapcore.OmitKey,
			MessageKey:     "msg",
			StacktraceKey:  "stacktrace",
			LineEnding:     zapcore.DefaultLineEnding,
			EncodeLevel:    zapcore.CapitalColorLevelEncoder,
			EncodeTime:     zapcore.ISO8601TimeEncoder,
			EncodeDuration: zapcore.StringDurationEncoder,
			EncodeCaller:   zapcore.ShortCallerEncoder,
		},
		DisableStacktrace: true,
		OutputPaths:       []string{filepath, "stdout"},
		ErrorOutputPaths:  []string{filepath, "stderr"},
	}.Build()
	if err != nil {
		return nil, err
	}

	return &logging.ZLogger{logger.Sugar().Named(name)}, nil
}
