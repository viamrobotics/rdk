package logging

import (
	"fmt"
	"testing"

	"go.viam.com/test"
)

var (
	l1 = NewLogger("logger-1")
	l2 = NewLogger("logger-2")
	l3 = NewLogger("logger-3")
)

func mockRegistry() *loggerRegistry {
	manager := newLoggerManager()
	manager.registerLogger("logger-1", l1)
	manager.registerLogger("logger-2", l2)
	l3.Sublogger("sublogger-1")
	loggerManager = manager
	return manager
}

func TestLoggerRegistration(t *testing.T) {
	manager := mockRegistry()

	expectedName := "test"
	expectedLogger := &impl{
		name:       expectedName,
		level:      NewAtomicLevelAt(INFO),
		appenders:  []Appender{NewStdoutAppender()},
		testHelper: func() {},
	}

	manager.registerLogger(expectedName, expectedLogger)

	actualLogger, ok := manager.loggerNamed("test")

	test.That(t, actualLogger, test.ShouldEqual, expectedLogger)
	test.That(t, ok, test.ShouldBeTrue)
}

func TestLoggerRetrieval(t *testing.T) {
	manager := mockRegistry()

	logger1, ok := manager.loggerNamed("logger-1")
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, logger1, test.ShouldEqual, l1)

	sublogger2, ok := manager.loggerNamed("sublogger-2")
	test.That(t, ok, test.ShouldBeFalse)
	test.That(t, sublogger2, test.ShouldBeNil)
}

func TestLoggersRegisteredOnCreation(t *testing.T) {
	manager := mockRegistry()

	newLogger := NewLogger("new")

	logger, ok := manager.loggerNamed("new")

	test.That(t, logger, test.ShouldEqual, newLogger)
	test.That(t, ok, test.ShouldBeTrue)

	debugLogger := NewDebugLogger("debug")

	logger, ok = manager.loggerNamed("debug")
	test.That(t, logger, test.ShouldEqual, debugLogger)
	test.That(t, ok, test.ShouldBeTrue)

	blankLogger := NewBlankLogger("blank")

	logger, ok = manager.loggerNamed("blank")
	test.That(t, logger, test.ShouldEqual, blankLogger)
	test.That(t, ok, test.ShouldBeTrue)

	sublogger := l1.Sublogger("sublogger-1")
	subloggerName := fmt.Sprintf("%s.%s", "logger-1", "sublogger-1")

	logger, ok = manager.loggerNamed(subloggerName)
	test.That(t, logger, test.ShouldEqual, sublogger)
	test.That(t, ok, test.ShouldBeTrue)
}

func TestUpdateLogLevel(t *testing.T) {
	manager := mockRegistry()

	err := manager.updateLoggerLevel("logger-1", DEBUG)
	test.That(t, err, test.ShouldBeNil)

	logger1, ok := manager.loggerNamed("logger-1")
	test.That(t, ok, test.ShouldBeTrue)
	level := logger1.GetLevel()
	test.That(t, level, test.ShouldEqual, DEBUG)

	err = manager.updateLoggerLevel("slogger-1", DEBUG)
	test.That(t, err, test.ShouldNotBeNil)
}
