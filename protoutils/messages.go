// protoutils are a collection of util methods for using proto in rdk
package protoutils

import (
	"reflect"

	"github.com/pkg/errors"
	commonpb "go.viam.com/rdk/proto/api/common/v1"
	"go.viam.com/rdk/resource"
)

// ResourceNameToProto converts a resource.Name to its proto counterpart.
func ResourceNameToProto(name resource.Name) *commonpb.ResourceName {
	return &commonpb.ResourceName{
		Uuid:      name.UUID,
		Namespace: string(name.Namespace),
		Type:      string(name.ResourceType),
		Subtype:   string(name.ResourceSubtype),
		Name:      name.Name,
	}
}

// ResourceNameFromProto converts a proto ResourceName to its rdk counterpart.
func ResourceNameFromProto(name *commonpb.ResourceName) resource.Name {
	return resource.NewName(
		resource.Namespace(name.Namespace),
		resource.TypeName(name.Type),
		resource.SubtypeName(name.Subtype),
		name.Name,
	)
}

// StructToMap attempts to coerce data into a form acceptable by grpc
func StructToMap(data interface{}) (map[string]interface{}, error) {
	res := map[string]interface{}{}
	if data == nil {
		return res, nil
	}
	v := reflect.TypeOf(data)
	reflectValue := reflect.ValueOf(data)
	reflectValue = reflect.Indirect(reflectValue)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}
	var err error
	for i := 0; i < v.NumField(); i++ {
		tag := v.Field(i).Tag.Get("json")
		field := reflectValue.Field(i).Interface()

		key := v.Field(i).Name
		if tag != "" && tag != "-" {
			key = tag
		}

		if v.Field(i).Type.Kind() == reflect.Struct {
			res[key], err = StructToMap(field)
			if err != nil {
				return nil, err
			}
		} else if v.Field(i).Type.Kind() == reflect.Slice {
			res[key], err = MarshalSlice(field)
			if err != nil {
				return nil, err
			}
		} else {
			res[key] = field
		}
	}
	return res, nil
}

// MarshalSlice attempts to coerce list data into a form acceptable by grpc
func MarshalSlice(lst interface{}) ([]interface{}, error) {
	s := reflect.ValueOf(lst)
	if s.Kind() != reflect.Slice {
		return nil, errors.New("input is not a slice")
	}

	newField := make([]interface{}, 0, s.Len())
	for i := 0; i < s.Len(); i++ {
		value := s.Index(i).Interface()
		v := reflect.TypeOf(value)
		reflectValue := reflect.ValueOf(value)
		reflectValue = reflect.Indirect(reflectValue)
		if v.Kind() == reflect.Ptr {
			v = v.Elem()
		}
		switch v.Kind() {
		case reflect.Struct:
			newData, err := StructToMap(value)
			if err != nil {
				return nil, err
			}
			newField = append(newField, newData)

		case reflect.Slice:
			newData, err := MarshalSlice(value)
			if err != nil {
				return nil, err
			}
			newField = append(newField, newData)
		default:
			newField = append(newField, value)
		}
	}
	return newField, nil
}
