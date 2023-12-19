package logging

import (
	"bytes"
	"context"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"go.viam.com/test"
)

type BasicStruct struct {
	X int
	y string
	z string
}

type User struct {
	Name string
}

type StructWithStruct struct {
	x int
	Y User
	z string
}

type StructWithAnonymousStruct struct {
	x int
	Y struct {
		Y1 string
	}
	Z string
}

// A custom `test.That` assertion function that takes the log line as a third argument for
// outputting a better message and a fourth for outputting the log line being tested.
func EqualsWithLogLine(actual interface{}, otherArgs ...interface{}) string {
	if len(otherArgs) != 3 {
		panic("EqualsWithMessage requires 4 inputs: actual, expected, message, log line.")
	}

	expected := otherArgs[0]
	message := otherArgs[1]
	logLine := otherArgs[2]
	if reflect.DeepEqual(actual, expected) {
		return ""
	}

	// A modified version of `assertions.ShouldEqual`.
	return fmt.Sprintf("Expected: '%v'\nActual:   '%v'\n(Should be equal)\nMessage: %v\nLog line: `%v`", expected, actual, message, logLine)
}

// assertLogMatches will fuzzy match log lines. Notably, this checks the time format, but ignores
// the exact time. And it expects a match on the filename, but the exact line number can be wrong.
func assertLogMatches(t *testing.T, actual *bytes.Buffer, expected string) {
	// `Helper` will result in test failures being associated with the callers line number. It's
	// more useful to report which `assertLogMatches` call failed rather than which assertion
	// inside this function. Maybe.
	t.Helper()

	output, err := actual.ReadString('\n')
	test.That(t, err, test.ShouldBeNil)

	actualTrimmed := strings.TrimSuffix(output, "\n")
	actualParts := strings.Split(actualTrimmed, "\t")
	expectedParts := strings.Split(expected, "\t")
	partsIdx := 0

	// Example log:
	//   2023-10-30T09:12:09.459Z	ERROR	impl	logging/impl_test.go:200	Errorw log	{"traceKey":"foobar","key":"value"}
	//   ^1                         ^2      ^3      ^4                          ^5          ^6
	//   Date                       Level   Name    File/Line                   Message     Structured Data

	// Use the length of the first string as a weak verification of checking that the result looks like a date.
	test.That(t, len(actualParts[partsIdx]), EqualsWithLogLine, len(expectedParts[partsIdx]), "Date length mismatch", actualTrimmed)

	// Log level.
	partsIdx++
	test.That(t, actualParts[partsIdx], EqualsWithLogLine, expectedParts[partsIdx], "Log level mismatch", actualTrimmed)

	// Logger name.
	partsIdx++
	test.That(t, actualParts[partsIdx], EqualsWithLogLine, expectedParts[partsIdx], "Logger name mismatch", actualTrimmed)

	// Filename:line_number.
	partsIdx++
	actualFilename, actualLineNumber, found := strings.Cut(actualParts[partsIdx], ":")
	test.That(t, found, EqualsWithLogLine, true, "Missing colon on test output", actualTrimmed)

	// Verify the filename matches exactly.
	expectedFilename, _, found := strings.Cut(expectedParts[partsIdx], ":")
	test.That(t, found, EqualsWithLogLine, true, "Missing colon on expected output", expected)
	test.That(t, actualFilename, EqualsWithLogLine, expectedFilename, "Filename mismatch", actualTrimmed)
	// Verify the line number is in fact a number, but no more.
	_, err = strconv.Atoi(actualLineNumber)
	test.That(t, err, EqualsWithLogLine, nil, "Line number is not a number", actualTrimmed)

	// Log message.
	partsIdx++
	test.That(t, actualParts[partsIdx], EqualsWithLogLine, expectedParts[partsIdx], "Log message mismatch", actualTrimmed)

	// Structured logging with the "w" API. E.g: `Debugw` has an extra tab delimited output.
	test.That(t, len(actualParts), EqualsWithLogLine, len(expectedParts), "Structured log mismatch", actualTrimmed)
	if len(actualParts) == partsIdx+1 {
		// We hit the end of the list.
		return
	}

	partsIdx++
	test.That(t, actualParts[partsIdx], EqualsWithLogLine, expectedParts[partsIdx], "Structured log mismatch", actualTrimmed)
}

// This test asserts our logger matches the output produced by the following zap config:
//
//	zap := zap.Must(zap.Config{
//			Level:	  zap.NewAtomicLevelAt(zap.InfoLevel),
//			Encoding: "console",
//			EncoderConfig: zapcore.EncoderConfig{
//				TimeKey:		"ts",
//				LevelKey:		"level",
//				NameKey:		"logger",
//				CallerKey:		"caller",
//				FunctionKey:	zapcore.OmitKey,
//				MessageKey:		"msg",
//				StacktraceKey:	"stacktrace",
//				LineEnding:		zapcore.DefaultLineEnding,
//				EncodeLevel:	zapcore.CapitalLevelEncoder,
//				EncodeTime:		zapcore.ISO8601TimeEncoder,
//				EncodeDuration: zapcore.StringDurationEncoder,
//				EncodeCaller:	zapcore.ShortCallerEncoder,
//			},
//			DisableStacktrace: true,
//			OutputPaths:	   []string{"stdout"},
//			ErrorOutputPaths:  []string{"stderr"},
//	}.Build()).Sugar()
//
// E.g:
//
//	2023-10-30T09:12:09.459-0400	INFO	logging/impl_test.go:87	zap Info log
func TestConsoleOutputFormat(t *testing.T) {
	// A logger object that will write to the `notStdout` buffer.
	const inUTC = true
	notStdout := &bytes.Buffer{}
	impl := &impl{"impl", NewAtomicLevelAt(DEBUG), inUTC, []Appender{NewWriterAppender(notStdout)}}

	impl.Info("impl Info log")
	// Note the use of tabs between the date, level, file location and log line. The
	// `assertLogMatches` helper will also deal with the changes to the time/line number.
	assertLogMatches(t, notStdout,
		`2023-10-30T09:12:09.459Z	INFO	impl	logging/impl_test.go:67	impl Info log`)

	// Using `Infof` substitutes the tail arguments into the leading template string input.
	impl.Infof("impl %s log", "infof")
	assertLogMatches(t, notStdout,
		`2023-10-30T09:45:20.764Z	INFO	impl	logging/impl_test.go:131	impl infof log`)

	// Using `Infow` turns the tail arguments into a map for structured logging.
	impl.Infow("impl logw", "key", "value")
	assertLogMatches(t, notStdout,
		`2023-10-30T13:19:45.806Z	INFO	impl	logging/impl_test.go:132	impl logw	{"key":"value"}`)

	// A few examples of structs.
	impl.Infow("impl logw", "key", "val", "StructWithAnonymousStruct", StructWithAnonymousStruct{1, struct{ Y1 string }{"y1"}, "foo"})
	assertLogMatches(t, notStdout,
		//nolint:lll
		`2023-10-31T14:25:10.239Z	INFO	impl	logging/impl_test.go:148	impl logw	{"key":"val","StructWithAnonymousStruct":{"Y":{"Y1":"y1"},"Z":"foo"}}`)

	impl.Infow("StructWithStruct", "key", "val", "StructWithStruct", StructWithStruct{1, User{"alice"}, "foo"})
	assertLogMatches(t, notStdout,
		`2023-10-31T14:25:21.095Z	INFO	impl	logging/impl_test.go:153	StructWithStruct	{"key":"val","StructWithStruct":{"Y":{"Name":"alice"}}}`)

	impl.Infow("BasicStruct", "implOneKey", "1val", "BasicStruct", BasicStruct{1, "alice", "foo"})
	assertLogMatches(t, notStdout,
		`2023-10-31T14:25:29.927Z	INFO	impl	logging/impl_test.go:157	BasicStruct	{"implOneKey":"1val","BasicStruct":{"X":1}}`)

	// Define a completely anonymous struct.
	anonymousTypedValue := struct {
		x int
		y struct {
			Y1 string
		}
		Z string
	}{1, struct{ Y1 string }{"y1"}, "z"}

	// Even though `y.Y1` is public, it is not included in the output. It isn't a rule that must be
	// excluded. This is tested just as a description of the current behavior.
	impl.Infow("impl logw", "key", "val", "anonymous struct", anonymousTypedValue)
	assertLogMatches(t, notStdout,
		`2023-10-31T14:25:39.320Z	INFO	impl	logging/impl_test.go:172	impl logw	{"key":"val","anonymous struct":{"Z":"z"}}`)

	// Represent a struct as a string using `fmt.Sprintf`.
	impl.Infow("impl logw", "key", "val", "fmt.Sprintf", fmt.Sprintf("%+v", anonymousTypedValue))
	assertLogMatches(t, notStdout,
		`2023-10-31T14:25:49.124Z	INFO	impl	logging/impl_test.go:177	impl logw	{"key":"val","fmt.Sprintf":"{x:1 y:{Y1:y1} Z:z}"}`)
}

func TestContextLogging(t *testing.T) {
	ctxNoDebug := context.Background()

	// A logger object that will write to the `notStdout` buffer.
	const inUTC = true
	notStdout := &bytes.Buffer{}
	// The default log level is error.
	logger := &impl{"impl", NewAtomicLevelAt(ERROR), inUTC, []Appender{NewWriterAppender(notStdout)}}

	logger.CDebug(ctxNoDebug, "Debug log")
	test.That(t, notStdout.Len(), test.ShouldEqual, 0)

	traceKey := "foobar"
	ctxWithDebug := EnableDebugModeWithKey(ctxNoDebug, traceKey)
	logger.CDebug(ctxWithDebug, "Debug log")
	assertLogMatches(t, notStdout,
		`2023-10-30T09:12:09.459Z	DEBUG	impl	logging/impl_test.go:200	Debug log	{"traceKey":"foobar"}`)

	logger.CDebugf(ctxWithDebug, "Debugf log %v", "Debugf")
	assertLogMatches(t, notStdout,
		`2023-10-30T09:12:09.459Z	DEBUG	impl	logging/impl_test.go:200	Debugf log Debugf	{"traceKey":"foobar"}`)

	logger.CDebugw(ctxWithDebug, "Debugw log", "key", "value")
	assertLogMatches(t, notStdout,
		`2023-10-30T09:12:09.459Z	DEBUG	impl	logging/impl_test.go:200	Debugw log	{"traceKey":"foobar","key":"value"}`)

	// Run the same battery of tests on the "Info" loggers.
	logger.CInfo(ctxNoDebug, "Info log")
	test.That(t, notStdout.Len(), test.ShouldEqual, 0)

	logger.CInfo(ctxWithDebug, "Info log")
	assertLogMatches(t, notStdout,
		`2023-10-30T09:12:09.459Z	INFO	impl	logging/impl_test.go:200	Info log	{"traceKey":"foobar"}`)

	logger.CInfof(ctxWithDebug, "Infof log %v", "Infof")
	assertLogMatches(t, notStdout,
		`2023-10-30T09:12:09.459Z	INFO	impl	logging/impl_test.go:200	Infof log Infof	{"traceKey":"foobar"}`)

	logger.CInfow(ctxWithDebug, "Infow log", "key", "value")
	assertLogMatches(t, notStdout,
		`2023-10-30T09:12:09.459Z	INFO	impl	logging/impl_test.go:200	Infow log	{"traceKey":"foobar","key":"value"}`)

	// Run the same battery of tests on the "Warn" loggers.
	logger.CWarn(ctxNoDebug, "Warn log")
	test.That(t, notStdout.Len(), test.ShouldEqual, 0)

	logger.CWarn(ctxWithDebug, "Warn log")
	assertLogMatches(t, notStdout,
		`2023-10-30T09:12:09.459Z	WARN	impl	logging/impl_test.go:200	Warn log	{"traceKey":"foobar"}`)

	logger.CWarnf(ctxWithDebug, "Warnf log %v", "Warnf")
	assertLogMatches(t, notStdout,
		`2023-10-30T09:12:09.459Z	WARN	impl	logging/impl_test.go:200	Warnf log Warnf	{"traceKey":"foobar"}`)

	logger.CWarnw(ctxWithDebug, "Warnw log", "key", "value")
	assertLogMatches(t, notStdout,
		`2023-10-30T09:12:09.459Z	WARN	impl	logging/impl_test.go:200	Warnw log	{"traceKey":"foobar","key":"value"}`)

	// Run the same calls on the "CError*" loggers. Because "Error" is the log level, the context
	// isn't needed for logging. But we continue to assert that the `traceKey` is included.
	logger.CError(ctxWithDebug, "Error log")
	assertLogMatches(t, notStdout,
		`2023-10-30T09:12:09.459Z	ERROR	impl	logging/impl_test.go:200	Error log	{"traceKey":"foobar"}`)

	logger.CErrorf(ctxWithDebug, "Errorf log %v", "Errorf")
	assertLogMatches(t, notStdout,
		`2023-10-30T09:12:09.459Z	ERROR	impl	logging/impl_test.go:200	Errorf log Errorf	{"traceKey":"foobar"}`)

	logger.CErrorw(ctxWithDebug, "Errorw log", "key", "value")
	assertLogMatches(t, notStdout,
		`2023-10-30T09:12:09.459Z	ERROR	impl	logging/impl_test.go:200	Errorw log	{"traceKey":"foobar","key":"value"}`)
}
