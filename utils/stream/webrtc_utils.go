package stream

import (
	"github.com/echolabsinc/robotcore/utils/log"

	"github.com/pion/logging"
)

type webrtcLoggerFactory struct {
	logger log.Logger
}

type webrtcLogger struct {
	logger log.Logger
}

func (wl webrtcLogger) Trace(msg string) {
	wl.logger.Debug(msg)
}

func (wl webrtcLogger) Tracef(format string, args ...interface{}) {
	wl.logger.Debugf(format, args...)
}

func (wl webrtcLogger) Debug(msg string) {
	wl.logger.Debug(msg)
}

func (wl webrtcLogger) Debugf(format string, args ...interface{}) {
	wl.logger.Debugf(format, args...)
}

func (wl webrtcLogger) Info(msg string) {
	wl.logger.Info(msg)
}

func (wl webrtcLogger) Infof(format string, args ...interface{}) {
	wl.logger.Infof(format, args...)
}

func (wl webrtcLogger) Warn(msg string) {
	wl.logger.Warn(msg)
}

func (wl webrtcLogger) Warnf(format string, args ...interface{}) {
	wl.logger.Warnf(format, args...)
}

func (wl webrtcLogger) Error(msg string) {
	wl.logger.Error(msg)
}

func (wl webrtcLogger) Errorf(format string, args ...interface{}) {
	wl.logger.Errorf(format, args...)
}

func (wlf webrtcLoggerFactory) NewLogger(scope string) logging.LeveledLogger {
	return webrtcLogger{wlf.logger.Named(scope)}
}
