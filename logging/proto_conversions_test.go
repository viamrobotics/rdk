package logging

import (
	"errors"
	"math"
	"testing"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.viam.com/test"
)

func TestFieldConversion(t *testing.T) {
	t.Parallel()

	testTime, err := time.Parse(DefaultTimeFormatStr, "2024-03-01T11:39:40.897Z")
	test.That(t, err, test.ShouldBeNil)

	type s struct {
		Field1 string
		Field2 bool
	}
	testStruct := &s{"foo", true}

	testCases := []struct {
		field       zap.Field
		expectedVal any
	}{
		{
			field: zap.Field{
				Key:     "boolean",
				Type:    zapcore.BoolType,
				Integer: 0,
			},
			expectedVal: false,
		},
		{
			field: zap.Field{
				Key:     "duration",
				Type:    zapcore.DurationType,
				Integer: time.Hour.Nanoseconds(),
			},
			expectedVal: time.Hour,
		},
		{
			field: zap.Field{
				Key:     "float64",
				Type:    zapcore.Float64Type,
				Integer: 4608218246714312622,
			},
			expectedVal: float64(1.23),
		},
		{
			field: zap.Field{
				Key:     "big float64",
				Type:    zapcore.Float64Type,
				Integer: int64(math.Float64bits(math.MaxFloat64)),
			},
			expectedVal: math.MaxFloat64,
		},
		{
			field: zap.Field{
				Key:     "float32",
				Type:    zapcore.Float32Type,
				Integer: 1068037571,
			},
			expectedVal: float32(1.32),
		},
		{
			field: zap.Field{
				Key:     "big float32",
				Type:    zapcore.Float32Type,
				Integer: int64(math.Float32bits(math.MaxFloat32)),
			},
			expectedVal: float32(math.MaxFloat32),
		},
		{
			field: zap.Field{
				Key:     "int",
				Type:    zapcore.Int64Type,
				Integer: -1234567891011,
			},
			expectedVal: int64(-1234567891011),
		},
		{
			field: zap.Field{
				Key:     "uint",
				Type:    zapcore.Uint64Type,
				Integer: 1234567891011,
			},
			expectedVal: uint64(1234567891011),
		},
		{
			field: zap.Field{
				Key:    "string",
				Type:   zapcore.StringType,
				String: "foobar",
			},
			expectedVal: "foobar",
		},
		{
			field: zap.Field{
				Key:    "error",
				Type:   zapcore.ErrorType,
				String: "error message",
			},
			// Error types retain only their message. Stacks are contained in log.Stack.
			expectedVal: "error message",
		},
		{
			field: zap.Field{
				Key:       "time",
				Type:      zapcore.TimeType,
				Interface: time.Local,
				Integer:   testTime.UnixNano(),
			},
			// Ensure that UTC is used instead of the Local location from original time.
			expectedVal: testTime.In(time.UTC),
		},
		{
			field: zap.Field{
				Key:       "struct",
				Type:      zapcore.ReflectType,
				Interface: testStruct,
			},
			// Types of structs cannot actually be preserved; we convert to
			// map[string]interface{}.
			expectedVal: map[string]interface{}{"Field1": "foo", "Field2": true},
		},
	}
	for _, tc := range testCases {
		tc := tc
		t.Run(tc.field.Key, func(t *testing.T) {
			t.Parallel()

			field, err := FieldToProto(tc.field)
			test.That(t, err, test.ShouldBeNil)

			key, val, err := FieldKeyAndValueFromProto(field)
			test.That(t, err, test.ShouldBeNil)
			test.That(t, key, test.ShouldEqual, tc.field.Key)
			test.That(t, val, test.ShouldResemble, tc.expectedVal)
		})
	}
}

type writeParams struct {
	Entry  zapcore.Entry
	Fields []zapcore.Field
}

type replayAppender struct {
	logs []writeParams
}

func (app *replayAppender) Write(entry zapcore.Entry, fields []zapcore.Field) error {
	app.logs = append(app.logs, writeParams{entry, fields})
	return nil
}

func (app *replayAppender) Sync() error {
	return nil
}

func TestErrorTypeRoundtrip(t *testing.T) {
	testLogger := NewTestLogger(t)
	_ = testLogger

	// Create a logger with a special appender to capture the exact `zapcore.Field` objects being
	// passed around.
	replayLogger := NewBlankLogger("proto")

	appender := &replayAppender{}
	replayLogger.AddAppender(appender)

	// Log a `w` line with an error value.
	replayLogger.Infow("log message", "err", errors.New("error message"))
	testLogger.Infof("LogEntry: %#v", appender.logs)

	test.That(t, len(appender.logs), test.ShouldEqual, 1)
	test.That(t, len(appender.logs[0].Fields), test.ShouldEqual, 1)

	// Grab the `err: error` field off the replay appender.
	field := appender.logs[0].Fields[0]
	testLogger.Infof("Field: %#v", field)

	// Demonstrate that the encoder is happy with this raw field.
	jsonEncoder := zapcore.NewJSONEncoder(zapcore.EncoderConfig{SkipLineEnding: true})
	_, err := jsonEncoder.EncodeEntry(zapcore.Entry{}, []zapcore.Field{field})
	test.That(t, err, test.ShouldBeNil)

	// Serialize the field to proto.
	proto, err := FieldToProto(field)
	test.That(t, err, test.ShouldBeNil)

	// Deserialize the proto back into a zapcore.Field.
	testLogger.Infof("Proto: %#v", proto)
	roundTrip, err := FieldFromProto(proto)
	test.That(t, err, test.ShouldBeNil)
	testLogger.Infof("Roundtrip: %#v", roundTrip)

	// Prior to RSDK-9097, this encoder step would fail. As we've created a zapcore.Field.Type ==
	// error. But with an empty zapcore.Field.Interface.
	_, err = jsonEncoder.EncodeEntry(zapcore.Entry{}, []zapcore.Field{roundTrip})
	test.That(t, err, test.ShouldBeNil)

	// TODO: Do better by preserving the error type
	// test.That(t, roundTrip.Type, test.ShouldEqual, zapcore.ErrorType)
}
