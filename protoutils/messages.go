// Package protoutils are a collection of util methods for using proto in rdk
package protoutils

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"

	"github.com/golang/geo/r3"
	"github.com/pkg/errors"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/wrapperspb"

	commonpb "go.viam.com/rdk/proto/api/common/v1"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/spatialmath"
)

// ResourceNameToProto converts a resource.Name to its proto counterpart.
func ResourceNameToProto(name resource.Name) *commonpb.ResourceName {
	return &commonpb.ResourceName{
		Namespace: string(name.Namespace),
		Type:      string(name.ResourceType),
		Subtype:   string(name.ResourceSubtype),
		Name:      name.ShortName(),
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

// InterfaceToMap attempts to coerce an interface into a form acceptable by structpb.NewStruct.
// Expects a struct or a map-like object.
func InterfaceToMap(data interface{}) (map[string]interface{}, error) {
	if data == nil {
		return nil, errors.New("no data passed in")
	}
	t := reflect.TypeOf(data)
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	var res map[string]interface{}
	var err error
	switch t.Kind() {
	case reflect.Struct:
		res, err = structToMap(data)
		if err != nil {
			return nil, err
		}
	case reflect.Map:
		res, err = marshalMap(data)
		if err != nil {
			return nil, err
		}
	case reflect.Array, reflect.Bool, reflect.Chan, reflect.Complex128, reflect.Complex64, reflect.Float32,
		reflect.Float64, reflect.Func, reflect.Int, reflect.Int16, reflect.Int32, reflect.Int64, reflect.Int8,
		reflect.Interface, reflect.Invalid, reflect.Pointer, reflect.Slice, reflect.String, reflect.Uint,
		reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uint8, reflect.Uintptr, reflect.UnsafePointer:
		fallthrough
	default:
		return nil, errors.Errorf("data of type %T and kind %s not a struct or a map-like object", data, t.Kind().String())
	}
	return res, nil
}

// StructToStructPb converts an arbitrary Go struct to a *structpb.Struct. Only exported fields are included in the
// returned proto.
func StructToStructPb(i interface{}) (*structpb.Struct, error) {
	encoded, err := InterfaceToMap(i)
	if err != nil {
		return nil, errors.Wrap(err,
			fmt.Sprintf("unable to convert interface %v to a form acceptable to structpb.NewStruct", i))
	}
	ret, err := structpb.NewStruct(encoded)
	if err != nil {
		return nil, errors.Wrap(err, fmt.Sprintf("unable to construct structpb.Struct from map %v", encoded))
	}
	return ret, nil
}

// takes a go type and tries to make it a better type for converting to grpc.
func toInterface(data interface{}) (interface{}, error) {
	if data == nil {
		return data, nil
	}

	t := reflect.TypeOf(data)
	v := reflect.ValueOf(data)
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
		v = reflect.Indirect(v)
	}

	var newData interface{}
	var err error
	switch t.Kind() {
	case reflect.Struct:
		newData, err = structToMap(data)
		if err != nil {
			return nil, err
		}
	case reflect.Map:
		newData, err = marshalMap(data)
		if err != nil {
			return nil, err
		}
	case reflect.Slice:
		newData, err = marshalSlice(data)
		if err != nil {
			return nil, err
		}
	case reflect.String:
		newData = v.String()
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		newData = v.Uint()
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		newData = v.Int()
	case reflect.Array, reflect.Bool, reflect.Chan, reflect.Complex128, reflect.Complex64, reflect.Float32,
		reflect.Float64, reflect.Func, reflect.Interface, reflect.Invalid, reflect.Pointer,
		reflect.Uintptr, reflect.UnsafePointer:
		fallthrough
	default:
		newData = data
	}
	return newData, nil
}

func isEmptyValue(v reflect.Value) bool {
	switch v.Kind() {
	case reflect.Array, reflect.Map, reflect.Slice, reflect.String:
		return v.Len() == 0
	case reflect.Bool:
		return !v.Bool()
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return v.Int() == 0
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return v.Uint() == 0
	case reflect.Float32, reflect.Float64:
		return v.Float() == 0
	case reflect.Interface, reflect.Ptr:
		return v.IsNil()
	case reflect.Chan, reflect.Complex128, reflect.Complex64, reflect.Func, reflect.Invalid, reflect.Struct,
		reflect.UnsafePointer:
		fallthrough
	default:
		return false
	}
}

// structToMap attempts to coerce a struct into a form acceptable by grpc. Ignores omitempty tags.
func structToMap(data interface{}) (map[string]interface{}, error) {
	t := reflect.TypeOf(data)
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	if t.Kind() != reflect.Struct {
		return nil, errors.Errorf("data of type %T is not a struct", data)
	}
	res := map[string]interface{}{}
	value := reflect.ValueOf(data)
	if value.Kind() == reflect.Ptr && value.IsNil() {
		return res, nil
	}
	value = reflect.Indirect(value)
	for i := 0; i < t.NumField(); i++ {
		sField := t.Field(i)
		tag := sField.Tag.Get("json")
		key := sField.Name

		if tag == "-" {
			continue
		}

		// tag name should be first element of tag string
		tagName := strings.Split(tag, ",")[0]
		if tagName != "" {
			key = tagName
		}

		// skip unexported fields
		if !value.Field(i).CanInterface() {
			continue
		}

		field := value.Field(i).Interface()

		// If "omitempty" is specified in the tag, it ignores empty values.
		if strings.Contains(tag, "omitempty") && isEmptyValue(reflect.ValueOf(field)) {
			continue
		}

		data, err := toInterface(field)
		if err != nil {
			return nil, err
		}

		res[key] = data
	}
	return res, nil
}

// marshalMap attempts to coerce maps of string keys into a form acceptable by grpc.
func marshalMap(data interface{}) (map[string]interface{}, error) {
	s := reflect.ValueOf(data)
	if s.Kind() != reflect.Map {
		return nil, errors.Errorf("data of type %T is not a map", data)
	}

	iter := reflect.ValueOf(data).MapRange()
	result := map[string]interface{}{}
	var err error
	for iter.Next() {
		k := iter.Key()
		if k.Kind() != reflect.String {
			return nil, errors.Errorf("map keys of type %v are not strings", k.Kind())
		}
		v := iter.Value().Interface()
		result[k.String()], err = toInterface(v)
		if err != nil {
			return nil, err
		}
	}
	return result, nil
}

// marshalSlice attempts to coerce list data into a form acceptable by grpc.
func marshalSlice(data interface{}) ([]interface{}, error) {
	s := reflect.ValueOf(data)
	if s.Kind() != reflect.Slice {
		return nil, errors.Errorf("data of type %T is not a slice", data)
	}

	newList := make([]interface{}, 0, s.Len())
	for i := 0; i < s.Len(); i++ {
		value := s.Index(i).Interface()
		data, err := toInterface(value)
		if err != nil {
			return nil, err
		}
		newList = append(newList, data)
	}
	return newList, nil
}

// ConvertVectorProtoToR3 TODO.
func ConvertVectorProtoToR3(v *commonpb.Vector3) r3.Vector {
	if v == nil {
		return r3.Vector{}
	}
	return r3.Vector{X: v.X, Y: v.Y, Z: v.Z}
}

// ConvertVectorR3ToProto TODO.
func ConvertVectorR3ToProto(v r3.Vector) *commonpb.Vector3 {
	return &commonpb.Vector3{X: v.X, Y: v.Y, Z: v.Z}
}

// ConvertOrientationToProto TODO.
func ConvertOrientationToProto(o spatialmath.Orientation) *commonpb.Orientation {
	oo := &commonpb.Orientation{}
	if o != nil {
		ov := o.OrientationVectorDegrees()
		oo.OX = ov.OX
		oo.OY = ov.OY
		oo.OZ = ov.OZ
		oo.Theta = ov.Theta
	}
	return oo
}

// ConvertProtoToOrientation TODO.
func ConvertProtoToOrientation(o *commonpb.Orientation) spatialmath.Orientation {
	return &spatialmath.OrientationVectorDegrees{
		OX:    o.OX,
		OY:    o.OY,
		OZ:    o.OZ,
		Theta: o.Theta,
	}
}

// ConvertStringToAnyPB takes a string and parses it to an Any pb type.
func ConvertStringToAnyPB(str string) (*anypb.Any, error) {
	var wrappedVal protoreflect.ProtoMessage
	if boolVal, err := strconv.ParseBool(str); err == nil {
		wrappedVal = wrapperspb.Bool(boolVal)
	} else if int64Val, err := strconv.ParseInt(str, 10, 64); err == nil {
		wrappedVal = wrapperspb.Int64(int64Val)
	} else if uint64Val, err := strconv.ParseUint(str, 10, 64); err == nil {
		wrappedVal = wrapperspb.UInt64(uint64Val)
	} else if float64Val, err := strconv.ParseFloat(str, 64); err == nil {
		wrappedVal = wrapperspb.Double(float64Val)
	} else {
		wrappedVal = wrapperspb.String(str)
	}
	anyVal, err := anypb.New(wrappedVal)
	if err != nil {
		return nil, err
	}
	return anyVal, nil
}

// ConvertStringMapToAnyPBMap takes a string map and parses each value to an Any proto type.
func ConvertStringMapToAnyPBMap(params map[string]string) (map[string]*anypb.Any, error) {
	methodParams := map[string]*anypb.Any{}
	for key, paramVal := range params {
		anyVal, err := ConvertStringToAnyPB(paramVal)
		if err != nil {
			return nil, err
		}
		methodParams[key] = anyVal
	}
	return methodParams, nil
}
