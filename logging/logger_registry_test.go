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

func TestPatternsConfig(t *testing.T) {
	rootLogger, registry := NewLoggerWithRegistry("root")
	err := registry.Update([]LoggerPatternConfig{
		{
			// `root.networking` and children are debug -- this unintentionally includes
			// `root.networkingfoo`. Which is expected to be a non-issue. And for the unexpected
			// case where it matters, it can be worked around by using two pattern configurations. A
			// `root.networking` and `root.networking.*` pattern.
			Pattern: "root.networking*",
			Level:   "debug",
		},
		{
			Pattern: "root.networking.webrtc*", // Except `root.networking.webrtc` and children are info
			Level:   "info",
		},
	}, NewLogger("error-logger"))
	test.That(t, err, test.ShouldBeNil)

	networkingLogger := rootLogger.Sublogger("networking")
	networkingChildLogger := networkingLogger.Sublogger("child")
	webrtcLogger := networkingLogger.Sublogger("webrtc")
	webrtcChildLogger := webrtcLogger.Sublogger("child")

	test.That(t, rootLogger.GetLevel(), test.ShouldEqual, INFO)
	test.That(t, networkingLogger.GetLevel(), test.ShouldEqual, DEBUG)
	test.That(t, networkingChildLogger.GetLevel(), test.ShouldEqual, DEBUG)
	test.That(t, webrtcLogger.GetLevel(), test.ShouldEqual, INFO)
	test.That(t, webrtcChildLogger.GetLevel(), test.ShouldEqual, INFO)
}

func TestPatternsEverythingWarnExcept(t *testing.T) {
	rootLogger, registry := NewLoggerWithRegistry("root")
	err := registry.Update([]LoggerPatternConfig{
		{
			Pattern: "*",
			Level:   "warn",
		},
		{
			Pattern: "root.networking",
			Level:   "info",
		},
	}, NewLogger("error-logger"))
	test.That(t, err, test.ShouldBeNil)

	networkingLogger := rootLogger.Sublogger("networking")
	subNetworkingLogger := networkingLogger.Sublogger("sub")
	otherLogger := rootLogger.Sublogger("other")

	test.That(t, rootLogger.GetLevel(), test.ShouldEqual, WARN)
	test.That(t, networkingLogger.GetLevel(), test.ShouldEqual, INFO)
	// `root.networking.sub` does not match `^root.networking$`. It only matches `*` and therefore
	// has a warn level.
	test.That(t, subNetworkingLogger.GetLevel(), test.ShouldEqual, WARN)
	test.That(t, otherLogger.GetLevel(), test.ShouldEqual, WARN)
}

func TestPatternsRootWarn(t *testing.T) {
	rootLogger, registry := NewLoggerWithRegistry("root")
	prePatternLogger := rootLogger.Sublogger("pre")
	err := registry.Update([]LoggerPatternConfig{
		{
			Pattern: "root",
			Level:   "warn",
		},
	}, NewLogger("error-logger"))
	test.That(t, err, test.ShouldBeNil)

	// RSDK-10893: If Subloggers were to inherit from their parent logger, this test would behave
	// differently based on when the "sublogger" is constructed. With inheritance and
	// constructing before patterns, "sublogger":
	// 1. `root` logger initializes as INFO.
	// 2. "sublogger" inherits INFO.
	// 3. Log pattern is applied to `^root$`. Only `root` is changed to WARN.
	//
	// Compared to:
	// 1. `root` logger initializes as INFO.
	// 2. Log pattern is applied to `^root$`. `root` is changed to WARN.
	// 3. "sublogger" inherits WARN.
	postPatternLogger := rootLogger.Sublogger("post")

	test.That(t, rootLogger.GetLevel(), test.ShouldEqual, WARN)
	// We assert that the subloggers are at the INFO level for clarity. This assertion is stronger
	// than what we actually care about. We actually only care that the pre and post logger levels
	// are equal.
	test.That(t, prePatternLogger.GetLevel(), test.ShouldEqual, INFO)
	test.That(t, postPatternLogger.GetLevel(), test.ShouldEqual, INFO)
}

func TestPatternsStarSuffix(t *testing.T) {
	rootLogger, registry := NewLoggerWithRegistry("root")
	err := registry.Update([]LoggerPatternConfig{
		{
			Pattern: "*.ftdc",
			Level:   "debug",
		},
		{
			Pattern: "*.rtsp-1",
			Level:   "debug",
		},
	}, NewLogger("error-logger"))
	test.That(t, err, test.ShouldBeNil)

	ftdcLogger := rootLogger.Sublogger("ftdc")
	// A logger that does not match any parents, although its parent logger did.
	subFTDCLogger := ftdcLogger.Sublogger("sub")
	rtspLogger := rootLogger.Sublogger("rtsp-1")
	test.That(t, rootLogger.GetLevel(), test.ShouldEqual, INFO)
	test.That(t, ftdcLogger.GetLevel(), test.ShouldEqual, DEBUG)
	// We assert that loggers get their level from patterns. Not their parents.
	test.That(t, subFTDCLogger.GetLevel(), test.ShouldEqual, INFO)
	test.That(t, rtspLogger.GetLevel(), test.ShouldEqual, DEBUG)
}
