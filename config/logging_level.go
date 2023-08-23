package config

import (
	"sync"

	"github.com/edaniels/golog"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var globalLogger struct {
	// These variables are initialized once at startup. No need for special synchronization.
	logger           golog.Logger
	cmdLineDebugFlag bool
	logLevel         zap.AtomicLevel

	// These variables can be changed while the `viam-server` is running. Additionally, every time one
	// of these is changed, we re-evaluate the log level. This mutex synchronizes the reads and writes
	// that can concurrently happen.
	mu                   sync.Mutex
	fileConfigDebugFlag  bool
	cloudConfigDebugFlag bool
	currentLevel         zapcore.Level
}

func InitLoggingSettings(logger golog.Logger, cmdLineDebugFlag bool, logLevel zap.AtomicLevel) {
	globalLogger.logger = logger
	globalLogger.cmdLineDebugFlag = cmdLineDebugFlag
	globalLogger.logLevel = logLevel
	globalLogger.currentLevel = logLevel.Level()
	globalLogger.logger.Info("Log level initialized: ", globalLogger.currentLevel)
}

func UpdateFileConfigDebug(fileDebug bool) {
	globalLogger.mu.Lock()
	defer globalLogger.mu.Unlock()

	globalLogger.fileConfigDebugFlag = fileDebug
	refreshLogLevelInLock()
}

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

	if globalLogger.currentLevel == newLevel {
		return
	}
	globalLogger.logger.Info("New log level: ", newLevel)
	globalLogger.logLevel.SetLevel(newLevel)
	globalLogger.currentLevel = newLevel
}
