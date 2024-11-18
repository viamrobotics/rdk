package data

import (
	"fmt"

	"go.mongodb.org/mongo-driver/bson"
	"google.golang.org/protobuf/types/known/structpb"
)

// pbStructToBSON converts a structpb.Struct to a bson.M.
func pbStructToBSON(s *structpb.Struct) (bson.M, error) {
	bsonMap := make(bson.M)
	for k, v := range s.Fields {
		bsonValue, err := convertPBStructValueToBSON(v)
		if err != nil {
			return nil, err
		}
		bsonMap[k] = bsonValue
	}
	return bsonMap, nil
}

func convertPBStructValueToBSON(v *structpb.Value) (interface{}, error) {
	switch v.Kind.(type) {
	case *structpb.Value_NullValue:
		var ret interface{}
		return ret, nil
	case *structpb.Value_NumberValue:
		return v.GetNumberValue(), nil
	case *structpb.Value_StringValue:
		return v.GetStringValue(), nil
	case *structpb.Value_BoolValue:
		return v.GetBoolValue(), nil
	case *structpb.Value_StructValue:
		return pbStructToBSON(v.GetStructValue())
	case *structpb.Value_ListValue:
		list := v.GetListValue()
		var slice bson.A
		for _, item := range list.Values {
			bsonValue, err := convertPBStructValueToBSON(item)
			if err != nil {
				return nil, err
			}
			slice = append(slice, bsonValue)
		}
		return slice, nil
	default:
		return nil, fmt.Errorf("unsupported value type: %T", v.Kind)
	}
}
