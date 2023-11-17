// Package data contains the code for automatically collecting readings from robots.
package data

import (
	"google.golang.org/protobuf/types/known/structpb"
)

// GetExpectedReadingsStruct converts a map[string]any into the structpb.Struct format
// expected for a Readings collector.
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
