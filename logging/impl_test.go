package logging

import (
	"bytes"
	"encoding/json"
	"fmt"
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
	// Use the length of the first string as a weak verification of checking that the result looks like a date.
	test.That(t, len(actualParts[0]), test.ShouldEqual, len(expectedParts[0]))
	// Log level.
	test.That(t, actualParts[1], test.ShouldEqual, expectedParts[1])

	// Filename:line_number.
	actualFilename, actualLineNumber, found := strings.Cut(actualParts[2], ":")
	test.That(t, found, test.ShouldBeTrue)
	// Verify the filename matches exactly.
	expectedFilename, _, found := strings.Cut(expectedParts[2], ":")
	test.That(t, found, test.ShouldBeTrue)
	test.That(t, actualFilename, test.ShouldEqual, expectedFilename)
	// Verify the line number is in fact a number, but no more.
	_, err = strconv.Atoi(actualLineNumber)
	test.That(t, err, test.ShouldBeNil)

	// Log message.
	test.That(t, actualParts[3], test.ShouldEqual, expectedParts[3])

	// Structured logging with the "w" API. E.g: `Debugw` has an extra tab delimited output.
	test.That(t, len(actualParts), test.ShouldEqual, len(expectedParts))
	if len(actualParts) == 4 {
		return
	}

	// JSON encoding of maps can be unpredictable because map iteration order can change between
	// runs. Parse the output into maps and assert on map equality.
	expectedMap := make(map[string]any)
	err = json.Unmarshal([]byte(expectedParts[4]), &expectedMap)
	test.That(t, err, test.ShouldBeNil)

	actualMap := make(map[string]any)
	err = json.Unmarshal([]byte(actualParts[4]), &actualMap)
	test.That(t, err, test.ShouldBeNil)

	test.That(t, actualMap, test.ShouldResemble, expectedMap)
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
	notStdout := &bytes.Buffer{}
	impl := &impl{"impl", DEBUG, notStdout}

	impl.Info("impl Info log")
	// Note the use of tabs between the date, level, file location and log line. The
	// `assertLogMatches` helper will also deal with the changes to the time/line number.
	assertLogMatches(t, notStdout,
		`2023-10-30T09:12:09.459-0400	INFO	logging/impl_test.go:67	impl Info log`)

	// Using `Infof` substitutes the tail arguments into the leading template string input.
	impl.Infof("impl %s log", "infof")
	assertLogMatches(t, notStdout,
		`2023-10-30T09:45:20.764-0400	INFO	logging/impl_test.go:131	impl infof log`)

	// Using `Infow` turns the tail arguments into a map for structured logging.
	impl.Infow("impl logw", "key", "value")
	assertLogMatches(t, notStdout,
		`2023-10-30T13:19:45.806-0400	INFO	logging/impl_test.go:132	impl logw	{"key":"value"}`)

	// A few examples of structs.
	impl.Infow("impl logw", "key", "val", "StructWithAnonymousStruct", StructWithAnonymousStruct{1, struct{ Y1 string }{"y1"}, "foo"})
	//nolint:lll
	assertLogMatches(t, notStdout,
		`2023-10-30T13:20:47.129-0400	INFO	logging/impl_test.go:121	impl logw	{"StructWithAnonymousStruct":{"Y":{"Y1":"y1"},"Z":"foo"},"key":"val"}`)

	impl.Infow("StructWithStruct", "key", "val", "StructWithStruct", StructWithStruct{1, User{"alice"}, "foo"})
	assertLogMatches(t, notStdout,
		`2023-10-30T13:20:47.129-0400	INFO	logging/impl_test.go:123	StructWithStruct	{"StructWithStruct":{"Y":{"Name":"alice"}},"key":"val"}`)

	impl.Infow("BasicStruct", "implOneKey", "1val", "BasicStruct", BasicStruct{1, "alice", "foo"})
	assertLogMatches(t, notStdout,
		`2023-10-30T13:20:47.129-0400	INFO	logging/impl_test.go:125	BasicStruct	{"BasicStruct":{"X":1},"implOneKey":"1val"}`)

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
		`2023-10-30T13:20:47.129-0400	INFO	logging/impl_test.go:119	impl logw	{"anonymous struct":{"Z":"z"},"key":"val"}`)

	// Represent a struct as a string using `fmt.Sprintf`.
	impl.Infow("impl logw", "key", "val", "fmt.Sprintf", fmt.Sprintf("%+v", anonymousTypedValue))
	assertLogMatches(t, notStdout,
		`2023-10-30T13:20:47.129-0400	INFO	logging/impl_test.go:127	impl logw	{"fmt.Sprintf":"{x:1 y:{Y1:y1} Z:z}","key":"val"}`)
}
