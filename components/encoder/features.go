// Package encoder contains an enum representing optional encoder features
package encoder

import (
	pb "go.viam.com/api/component/encoder/v1"
)

// Feature is an enum representing an optional motor feature.
type Feature string

// PositionReporting represesnts the feature of a motor being
// able to report its own position.
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
