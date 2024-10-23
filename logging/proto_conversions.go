package logging

import (
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.viam.com/utils/protoutils"
	"google.golang.org/protobuf/types/known/structpb"
)

// FieldToProto converts a zap.Field to a *structpb.Struct.
func FieldToProto(field zap.Field) (*structpb.Struct, error) {
	// Zap encodes float64s with very large int64s. Proto conversions have some
	// loss with very large int64s. float32s are also encoded with int64s, but
	// the int64 encodings are not large enough to cause loss in conversion.
	// See https://pkg.go.dev/google.golang.org/protobuf@v1.32.0/types/known/structpb#NewValue.
	//
	// Use a hacky combination of fmt and math to store float64s as strings.
	if field.Type == zapcore.Float64Type {
		field.String = fmt.Sprintf("%f", math.Float64frombits(uint64(field.Integer)))
	}

	// RSDK-9097: Calling FieldToProto goes through a marshal -> unmarshal sequence. Marshaling
	// errors as-is will give us a zapcore type of error, but an empty `String` and an empty object
	// for the `Interface`. Unmarshalling in `FieldFromProto` subsequently creates a zapcore.Field
	// object with the "error" type, but no underlying `Interface`. Using the zapcore JSONEncoder
	// fails when the underlying empty `Interface` is cast to an `error`.
	//
	// We work around this in the short-term by just turning errors into strings.
	if field.Type == zapcore.ErrorType {
		field.Type = zapcore.StringType
		field.String = field.Interface.(error).Error()
	}

	return protoutils.StructToStructPb(field)
}

// FieldFromProto unmarshals a proto-encoded zap.Field.
func FieldFromProto(field *structpb.Struct) (zap.Field, error) {
	fieldJSON, err := json.Marshal(field)
	if err != nil {
		return zap.Field{}, err
	}

	var zf zap.Field
	if err := json.Unmarshal(fieldJSON, &zf); err != nil {
		return zap.Field{}, err
	}

	// Handle poorly serialized error fields (force them into a string type with
	// an empty value). Newer Golang modules should serialize correctly (turn
	// errors into strings client-side) per RSDK-9097.
	if zf.Type == zapcore.ErrorType {
		if _, ok := zf.Interface.(error); !ok {
			zf.Type = zapcore.StringType
			zf.String = ""
		}
	}

	return zf, err
}

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
		// See FieldToProto above: we encode float64s as strings to avoid loss in
		// proto conversion. Old logs from the app DB may still have float64s in
		// the Integer field: return that field casted to float64 in such cases.
		if zf.String == "" {
			fieldValue = math.Float64frombits(uint64(zf.Integer))
			break
		}

		fieldValue, err = strconv.ParseFloat(zf.String, 64)
		if err != nil {
			return "", nil, err
		}
	case zapcore.Float32Type:
		fieldValue = math.Float32frombits(uint32(zf.Integer))
	case zapcore.Int64Type, zapcore.Int32Type, zapcore.Int16Type, zapcore.Int8Type:
		fieldValue = zf.Integer
	case zapcore.Uint64Type, zapcore.Uint32Type, zapcore.Uint16Type, zapcore.Uint8Type:
		fieldValue = uint64(zf.Integer)
	case zapcore.StringType, zapcore.ErrorType:
		fieldValue = zf.String
	case zapcore.TimeType:
		// Ignore *time.Location stored in zf.Interface; we'll force UTC.
		fieldValue = time.Unix(0, zf.Integer).In(time.UTC)
	default:
		fieldValue = zf.Interface
	}

	return zf.Key, fieldValue, nil
}
