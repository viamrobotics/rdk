package data

import (
	"reflect"

	geo "github.com/kellydunn/golang-geo"
	"google.golang.org/protobuf/types/known/structpb"
)

// GetExpectedReadingsStruct converts a map[string]any into the structpb.Struct format
// expected for a Readings collector. Used in tests.
func GetExpectedReadingsStruct(data map[string]any) *structpb.Struct {
	readings := make(map[string]*structpb.Value)
	for name, value := range data {
		//nolint:errcheck
		val, _ := structpb.NewValue(value)
		readings[name] = val
	}

	topLevelMap := make(map[string]*structpb.Value)
	topLevelMap["readings"] = structpb.NewStructValue(
		&structpb.Struct{Fields: readings},
	)
	return &structpb.Struct{Fields: topLevelMap}
}

// StructValueMapFromInterfaceMap converts a map[string]interface{} to a map[string]*structpb.Value
// format. This is used to convert the map returned by the Readings Go API into the readings in
// the GetReadingsResponse that a Readings collector returns.
func StructValueMapFromInterfaceMap(values map[string]interface{}) (map[string]*structpb.Value, error) {
	res := make(map[string]*structpb.Value)
	for name, value := range values {
		// Since geo.Point is an external dependency with unexported fields, cannot use reflection.
		if decodedGeoPoint, ok := value.(*geo.Point); ok {
			lat, err := structpb.NewValue(decodedGeoPoint.Lat())
			if err != nil {
				return nil, err
			}
			lng, err := structpb.NewValue(decodedGeoPoint.Lng())
			if err != nil {
				return nil, err
			}
			res[name] = structpb.NewStructValue(&structpb.Struct{
				Fields: map[string]*structpb.Value{"lat": lat, "lng": lng},
			})
			continue
		}

		// This handles all structs and struct pointers with exported fields, e.g. *spatialmath.Quaternion and r3.Vector.
		reflectValue := reflect.ValueOf(value)
		if reflectValue.Kind() == reflect.Struct || (reflectValue.Kind() == reflect.Ptr && reflectValue.Elem().Kind() == reflect.Struct) {
			valueInterfaceMap := make(map[string]interface{})
			// If it's a pointer, get the actual struct.
			if reflectValue.Kind() == reflect.Ptr {
				reflectValue = reflectValue.Elem()
			}
			for i := 0; i < reflectValue.NumField(); i++ {
				fieldName := reflectValue.Type().Field(i).Name
				fieldValue := reflectValue.Field(i).Interface()
				valueInterfaceMap[fieldName] = fieldValue
			}
			structValueMap, err := StructValueMapFromInterfaceMap(valueInterfaceMap)
			if err != nil {
				return nil, err
			}
			res[name] = structpb.NewStructValue(&structpb.Struct{Fields: structValueMap})
			continue
		}

		val, err := structpb.NewValue(value)
		if err != nil {
			return nil, err
		}
		res[name] = val
	}
	return res, nil
}
