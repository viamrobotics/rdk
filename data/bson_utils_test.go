package data

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/google/uuid"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.viam.com/test"
	"google.golang.org/protobuf/types/known/structpb"
)

// bsonToStructPB converts a bson.M to a structpb.Struct.
func bsonToStructPB(bsonMap bson.M) (*structpb.Struct, error) {
	s := &structpb.Struct{
		Fields: make(map[string]*structpb.Value),
	}
	for k, v := range bsonMap {
		value, err := convertBSONValueToStructPBValue(v)
		if err != nil {
			return nil, err
		}
		s.Fields[k] = value
	}
	return s, nil
}

func convertBSONValueToStructPBValue(v interface{}) (*structpb.Value, error) {
	switch val := v.(type) {
	case nil, primitive.Undefined:
		return &structpb.Value{Kind: &structpb.Value_NullValue{}}, nil
	case float64:
		return &structpb.Value{Kind: &structpb.Value_NumberValue{NumberValue: val}}, nil
	case int64:
		return &structpb.Value{Kind: &structpb.Value_NumberValue{NumberValue: float64(val)}}, nil
	case int32:
		return &structpb.Value{Kind: &structpb.Value_NumberValue{NumberValue: float64(val)}}, nil
	case string:
		return &structpb.Value{Kind: &structpb.Value_StringValue{StringValue: val}}, nil
	case bool:
		return &structpb.Value{Kind: &structpb.Value_BoolValue{BoolValue: val}}, nil
	case bson.M:
		s, err := bsonToStructPB(val)
		if err != nil {
			return nil, err
		}
		return &structpb.Value{Kind: &structpb.Value_StructValue{StructValue: s}}, nil
	case bson.A:
		list := &structpb.ListValue{}
		for _, item := range val {
			value, err := convertBSONValueToStructPBValue(item)
			if err != nil {
				return nil, err
			}
			list.Values = append(list.Values, value)
		}
		return &structpb.Value{Kind: &structpb.Value_ListValue{ListValue: list}}, nil
	case primitive.DateTime:
		return &structpb.Value{Kind: &structpb.Value_StringValue{StringValue: val.Time().String()}}, nil
	case primitive.Timestamp:
		jsonStr, err := json.Marshal(val)
		if err != nil {
			return nil, err
		}
		return &structpb.Value{Kind: &structpb.Value_StringValue{StringValue: string(jsonStr)}}, nil
	case primitive.JavaScript:
		return &structpb.Value{Kind: &structpb.Value_StringValue{StringValue: string(val)}}, nil
	case primitive.Symbol:
		return &structpb.Value{Kind: &structpb.Value_StringValue{StringValue: string(val)}}, nil
	case primitive.DBPointer, primitive.CodeWithScope, primitive.Decimal128, primitive.Regex, primitive.ObjectID:
		return &structpb.Value{Kind: &structpb.Value_StringValue{StringValue: val.(fmt.Stringer).String()}}, nil
	case primitive.MinKey:
		return &structpb.Value{Kind: &structpb.Value_StringValue{StringValue: "MinKey"}}, nil
	case primitive.MaxKey:
		return &structpb.Value{Kind: &structpb.Value_StringValue{StringValue: "MaxKey"}}, nil
	case primitive.Binary:
		// If it's a UUID, return the UUID as a hex string.
		if val.Subtype == bson.TypeBinaryUUID {
			data, err := uuid.FromBytes(val.Data)
			if err != nil {
				return nil, err
			}
			return &structpb.Value{Kind: &structpb.Value_StringValue{StringValue: data.String()}}, nil
		}

		// Otherwise return a list of the raw bytes.
		list := make([]*structpb.Value, len(val.Data))
		for i, b := range val.Data {
			list[i] = &structpb.Value{Kind: &structpb.Value_NumberValue{NumberValue: float64(b)}}
		}
		return &structpb.Value{Kind: &structpb.Value_ListValue{ListValue: &structpb.ListValue{Values: list}}}, nil
	default:
		return nil, fmt.Errorf("unsupported BSON type: %T", v)
	}
}

func TestBSONToStructPBAndBack(t *testing.T) {
	tests := []struct {
		name         string
		input        *structpb.Struct
		expectedBSON primitive.M
	}{
		{
			name: "Primitive fields are properly converted between structpb.Struct <-> BSON.",
			input: &structpb.Struct{
				Fields: map[string]*structpb.Value{
					"name":     {Kind: &structpb.Value_StringValue{StringValue: "John"}},
					"age":      {Kind: &structpb.Value_NumberValue{NumberValue: 30}},
					"alive":    {Kind: &structpb.Value_BoolValue{BoolValue: true}},
					"nullable": {Kind: &structpb.Value_NullValue{}},
				},
			},
			expectedBSON: bson.M{
				"name":     "John",
				"age":      30.0,
				"alive":    true,
				"nullable": nil,
			},
		},
		{
			name: "Nested struct fields are properly converted between structpb.Struct <-> BSON.",
			input: &structpb.Struct{
				Fields: map[string]*structpb.Value{
					"person": {
						Kind: &structpb.Value_StructValue{
							StructValue: &structpb.Struct{
								Fields: map[string]*structpb.Value{
									"name":  {Kind: &structpb.Value_StringValue{StringValue: "Alice"}},
									"age":   {Kind: &structpb.Value_NumberValue{NumberValue: 25}},
									"alive": {Kind: &structpb.Value_BoolValue{BoolValue: true}},
								},
							},
						},
					},
				},
			},
			expectedBSON: bson.M{
				"person": bson.M{
					"name":  "Alice",
					"age":   float64(25),
					"alive": true,
				},
			},
		},
		{
			name: "List fields are properly converted between structpb.Struct <-> BSON.",
			input: &structpb.Struct{
				Fields: map[string]*structpb.Value{
					"names": {
						Kind: &structpb.Value_ListValue{
							ListValue: &structpb.ListValue{
								Values: []*structpb.Value{
									{Kind: &structpb.Value_StringValue{StringValue: "Bob"}},
									{Kind: &structpb.Value_StringValue{StringValue: "Charlie"}},
								},
							},
						},
					},
				},
			},
			expectedBSON: bson.M{
				"names": bson.A{"Bob", "Charlie"},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Convert StructPB to BSON
			bsonMap, err := pbStructToBSON(tc.input)
			test.That(t, err, test.ShouldBeNil)

			// Validate the BSON is structured as expected.
			test.That(t, bsonMap, test.ShouldResemble, tc.expectedBSON)

			// Convert BSON back to StructPB
			result, err := bsonToStructPB(bsonMap)
			test.That(t, err, test.ShouldBeNil)

			// Check if the result matches the original input
			test.That(t, result, test.ShouldResemble, tc.input)
		})
	}
}
