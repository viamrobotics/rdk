// Package encoder contains an enum representing optional encoder features
package encoder

import (
	pb "go.viam.com/api/component/encoder/v1"
)

// Feature is an enum representing an optional encoder feature.
type Feature string

// TicksCountSupported and AngleDegreesSupported represesnts the feature
// of an encoder being able to report ticks and/or degrees, respectively.
const (
	TicksCountSupported   Feature = "Ticks"
	AngleDegreesSupported Feature = "Degrees"
)

// ProtoFeaturesToMap takes a GetPropertiesResponse and returns
// an equivalent Feature-to-boolean map.
func ProtoFeaturesToMap(resp *pb.GetPropertiesResponse) map[Feature]bool {
	return map[Feature]bool{
		TicksCountSupported:   resp.TicksCountSupported,
		AngleDegreesSupported: resp.AngleDegreesSupported,
	}
}

// FeatureMapToProtoResponse takes a map of features to booleans (indicating
// whether the feature is supported) and converts it to a GetPropertiesResponse.
func FeatureMapToProtoResponse(
	featureMap map[Feature]bool,
) (*pb.GetPropertiesResponse, error) {
	return &pb.GetPropertiesResponse{
		TicksCountSupported:   featureMap[TicksCountSupported],
		AngleDegreesSupported: featureMap[AngleDegreesSupported],
	}, nil
}
