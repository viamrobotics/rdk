// Package base contains an enum representing optional base features
package base

import pb "go.viam.com/api/component/base/v1"

// Feature is an enum representing an optional base feature.
type Feature string

// TurningRadiusM and WidthM represesnts the physical features
// of a base reporting its respective minimum turning radius and
// width respectively, in units of meters (M)
const (
	TurningRadiusM Feature = "TurningRadiusM"
	WidthM         Feature = "WidthM"
)

// ProtoFeaturesToMap takes a GetPropertiesResponse and returns
// an equivalent Feature-to-boolean map.
func ProtoFeaturesToMap(resp *pb.GetPropertiesResponse) map[Feature]float64 {
	return map[Feature]float64{
		TurningRadiusM: resp.TurningRadius,
		WidthM:         resp.WidthMm,
	}
}

// FeatureMapToProtoResponse takes a map of features to booleans (indicating
// whether the feature is supported) and converts it to a GetPropertiesResponse.
func FeatureMapToProtoResponse(
	featureMap map[Feature]float64,
) (*pb.GetPropertiesResponse, error) {
	return &pb.GetPropertiesResponse{
		TurningRadius: featureMap[TurningRadiusM],
		WidthMm:       featureMap[WidthM],
	}, nil
}
