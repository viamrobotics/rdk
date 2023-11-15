package data

import "google.golang.org/protobuf/types/known/structpb"

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
		val, err := structpb.NewValue(value)
		if err != nil {
			return nil, err
		}
		res[name] = val
	}
	return res, nil
}
