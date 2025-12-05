package config

import (
	"sync"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"go.viam.com/rdk/logging"
)

var globalLogger struct {
	// These variables are initialized once at startup. No need for special synchronization.
	logger           logging.Logger
	cmdLineDebugFlag bool

	// These variables can be changed while the `viam-server` is running. Additionally, every time one
	// of these is changed, we re-evaluate the log level. This mutex synchronizes the reads and writes
	// that can concurrently happen.
	mu                   sync.Mutex
	fileConfigDebugFlag  bool
	cloudConfigDebugFlag bool
}

// InitLoggingSettings initializes the global logging settings.
func InitLoggingSettings(logger logging.Logger, cmdLineDebugFlag bool) {
	globalLogger.mu.Lock()
	defer globalLogger.mu.Unlock()

	globalLogger.logger = logger
	globalLogger.cmdLineDebugFlag = cmdLineDebugFlag

	if cmdLineDebugFlag {
		logging.GlobalLogLevel.SetLevel(zapcore.DebugLevel)
		logger.SetLevel(logging.DEBUG)
	} else {
		logging.GlobalLogLevel.SetLevel(zapcore.InfoLevel)
		logger.SetLevel(logging.INFO)
	}

	globalLogger.logger.Info("Log level initialized:", logging.GlobalLogLevel.Level())
}

// UpdateFileConfigDebug is used to update the debug flag whenever a file-based viam config is
// refreshed.
func UpdateFileConfigDebug(fileDebug bool) {
	globalLogger.mu.Lock()
	defer globalLogger.mu.Unlock()

	globalLogger.fileConfigDebugFlag = fileDebug
	refreshLogLevelInLock()
}

// UpdateCloudConfigDebug is used to update the debug flag whenever a cloud-based viam config
// is refreshed.
func UpdateCloudConfigDebug(cloudDebug bool) {
	globalLogger.mu.Lock()
	defer globalLogger.mu.Unlock()

	globalLogger.cloudConfigDebugFlag = cloudDebug
	refreshLogLevelInLock()
}

func refreshLogLevelInLock() {
	// We have two loggers to update here: logging.GlobalLogLevel (zapcore) and globalLogger.logger (logging)
	// Also see usages of InitLoggingSettings.
	var newLevelZap zapcore.Level
	var newLevel logging.Level
	if globalLogger.cmdLineDebugFlag ||
		globalLogger.fileConfigDebugFlag ||
		globalLogger.cloudConfigDebugFlag {
		// If anything wants debug logs, set the level to `Debug`.
		newLevelZap = zap.DebugLevel
		newLevel = logging.DEBUG
	} else {
		// If none of the command line, file config or cloud config ask for debug, use the `Info` log
		// level.
		newLevelZap = zap.InfoLevel
		newLevel = logging.INFO
	}

	if logging.GlobalLogLevel.Level() == newLevelZap {
		return
	}
	globalLogger.logger.Info("New log level:", newLevelZap)

	logging.GlobalLogLevel.SetLevel(newLevelZap)
	globalLogger.logger.SetLevel(newLevel)
}
