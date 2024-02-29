package logging

import (
	"encoding/json"
	"errors"
	"math"
	"strconv"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"google.golang.org/protobuf/types/known/structpb"
)

// FieldKeyAndValueFromProto examines a *structpb.Struct and returns its key
// string and native golang value.
func FieldKeyAndValueFromProto(field *structpb.Struct) (string, any, error) {
	var fieldValue any

	fieldJSON, err := json.Marshal(field)
	if err != nil {
		return "", nil, err
	}

	var zf zap.Field
	if err := json.Unmarshal(fieldJSON, &zf); err != nil {
		return "", nil, err
	}

	// This code is modeled after zapcore.Field.AddTo:
	// https://github.com/uber-go/zap/blob/fcf8ee58669e358bbd6460bef5c2ee7a53c0803a/zapcore/field.go#L114
	//nolint:exhaustive
	switch zf.Type {
	case zapcore.BoolType:
		fieldValue = zf.Integer == 1
	case zapcore.DurationType:
		fieldValue = time.Duration(zf.Integer)
	case zapcore.Float64Type:
		// See robot/client/client.go: we encode float64s as strings to avoid loss
		// in proto conversion.
		if zf.String == "" {
			return "", nil, errors.New("must encode float64s in the String field")
		}
		fieldValue, err = strconv.ParseFloat(zf.String, 64)
		if err != nil {
			return "", nil, err
		}
	case zapcore.Float32Type:
		fieldValue = math.Float32frombits(uint32(zf.Integer))
	case zapcore.Int64Type, zapcore.Int32Type, zapcore.Int16Type, zapcore.Int8Type,
		zapcore.Uint64Type, zapcore.Uint32Type, zapcore.Uint16Type, zapcore.Uint8Type:
		fieldValue = zf.Integer
	case zapcore.StringType, zapcore.ErrorType:
		fieldValue = zf.String
	case zapcore.TimeType:
		// Ignore *time.Location stored in zf.Interface; we'll use the UTC default.
		fieldValue = time.Unix(0, zf.Integer)
	default:
		fieldValue = zf.Interface
	}

	return zf.Key, fieldValue, nil
}
