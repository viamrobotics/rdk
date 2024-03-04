package logging

import (
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
