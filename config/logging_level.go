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
	globalLogger.logger = logger
	globalLogger.cmdLineDebugFlag = cmdLineDebugFlag
	if cmdLineDebugFlag {
		logging.GlobalLogLevel.SetLevel(zapcore.DebugLevel)
	} else {
		logging.GlobalLogLevel.SetLevel(zapcore.InfoLevel)
	}
	globalLogger.logger.Info("Log level initialized: ", logging.GlobalLogLevel.Level())
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
	var newLevel zapcore.Level
	if globalLogger.cmdLineDebugFlag ||
		globalLogger.fileConfigDebugFlag ||
		globalLogger.cloudConfigDebugFlag {
		// If anything wants debug logs, set the level to `Debug`.
		newLevel = zap.DebugLevel
	} else {
		// If none of the command line, file config or cloud config ask for debug, use the `Info` log
		// level.
		newLevel = zap.InfoLevel
	}

	if logging.GlobalLogLevel.Level() == newLevel {
		return
	}
	globalLogger.logger.Info("New log level: ", newLevel)
	logging.GlobalLogLevel.SetLevel(newLevel)
}
