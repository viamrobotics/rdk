package logging

import (
	"testing"

	"go.viam.com/test"
)

var (
	l1 = NewLogger("logger-1")
	l2 = NewLogger("logger-2")
	l3 = NewLogger("logger-3")
)

func mockRegistry() *Registry {
	registry := newRegistry()
	registry.registerLogger("logger-1", l1)
	registry.registerLogger("logger-2", l2)
	l3.Sublogger("sublogger-1")
	return registry
}

func TestLoggerRegistration(t *testing.T) {
	registry := mockRegistry()

	expectedName := "test"
	expectedLogger := &impl{
		name:       expectedName,
		level:      NewAtomicLevelAt(INFO),
		appenders:  []Appender{NewStdoutAppender()},
		registry:   newRegistry(),
		testHelper: func() {},
	}

	registry.registerLogger(expectedName, expectedLogger)

	actualLogger, ok := registry.loggerNamed("test")

	test.That(t, actualLogger, test.ShouldEqual, expectedLogger)
	test.That(t, ok, test.ShouldBeTrue)
}

func TestLoggerRetrieval(t *testing.T) {
	registry := mockRegistry()

	logger1, ok := registry.loggerNamed("logger-1")
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, logger1, test.ShouldEqual, l1)

	sublogger2, ok := registry.loggerNamed("sublogger-2")
	test.That(t, ok, test.ShouldBeFalse)
	test.That(t, sublogger2, test.ShouldBeNil)
}

func TestUpdateLogLevel(t *testing.T) {
	registry := mockRegistry()

	err := registry.updateLoggerLevel("logger-1", DEBUG)
	test.That(t, err, test.ShouldBeNil)

	logger1, ok := registry.loggerNamed("logger-1")
	test.That(t, ok, test.ShouldBeTrue)
	level := logger1.GetLevel()
	test.That(t, level, test.ShouldEqual, DEBUG)

	err = registry.updateLoggerLevel("slogger-1", DEBUG)
	test.That(t, err, test.ShouldNotBeNil)
}

func TestGetRegisteredNames(t *testing.T) {
	registry := mockRegistry()
	for _, name := range registry.getRegisteredLoggerNames() {
		_, ok := registry.loggerNamed(name)
		test.That(t, ok, test.ShouldBeTrue)
	}
}

func TestRegisterConfig(t *testing.T) {
	registry := mockRegistry()
	fakeLogger := NewLogger("abc")
	registry.registerLogger("abc", fakeLogger)
	logCfg := []LoggerPatternConfig{
		{
			Pattern: "abc",
			Level:   "WARN",
		},
		{
			Pattern: "def",
			Level:   "ERROR",
		},
	}
	err := registry.Update(logCfg, NewLogger("error-logger"))
	test.That(t, err, test.ShouldBeNil)
	test.That(t, registry.logConfig, test.ShouldResemble, logCfg)
	test.That(t, fakeLogger.GetLevel().String(), test.ShouldEqual, "Warn")
}

func TestGetOrRegister(t *testing.T) {
	registry := mockRegistry()

	logCfg := []LoggerPatternConfig{
		{
			Pattern: "a.*",
			Level:   "WARN",
		},
	}
	err := registry.Update(logCfg, NewLogger("error-logger"))
	test.That(t, err, test.ShouldBeNil)

	_ = registry.getOrRegister("a.b.c", NewLogger("a.b.c"))
	loggerABD := registry.getOrRegister("a.b.d", NewLogger("a.b.d"))
	loggerABD2 := registry.getOrRegister("a.b.d", NewLogger("a.b.d"))

	// loggerABD and loggerABD2 should be identical.
	test.That(t, loggerABD, test.ShouldEqual, loggerABD2)

	// Both "a.b.c" and "a.b.d" loggers should be in registry and set to "Warn".
	logger, ok := registry.loggerNamed("a.b.c")
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, logger.GetLevel().String(), test.ShouldEqual, "Warn")

	logger, ok = registry.loggerNamed("a.b.d")
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, logger.GetLevel().String(), test.ShouldEqual, "Warn")
}

func TestGetCurrentConfig(t *testing.T) {
	registry := mockRegistry()
	logCfg := []LoggerPatternConfig{
		{
			Pattern: "a.*",
			Level:   "WARN",
		},
	}
	registry.Update(logCfg, NewLogger("error-logger"))
	test.That(t, registry.GetCurrentConfig(), test.ShouldResemble, logCfg)
}

func TestLoggerLevelReset(t *testing.T) {
	registry := mockRegistry()
	registry.registerLogger("a", NewLogger("a"))
	logCfg := []LoggerPatternConfig{
		{
			Pattern: "a",
			Level:   "WARN",
		},
	}

	err := registry.Update(logCfg, NewLogger("error-logger"))
	test.That(t, err, test.ShouldBeNil)

	logger, ok := registry.loggerNamed("a")
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, logger.GetLevel().String(), test.ShouldEqual, "Warn")

	logCfg = []LoggerPatternConfig{}

	err = registry.Update(logCfg, NewLogger("error-logger"))
	test.That(t, err, test.ShouldBeNil)

	logger, ok = registry.loggerNamed("a")
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, logger.GetLevel().String(), test.ShouldEqual, "Info")
}
