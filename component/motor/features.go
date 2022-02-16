// Package motor contains an enum representing optional motor features
package motor

import (
	pb "go.viam.com/rdk/proto/api/component/v1"
)

// Feature is an enum representing an optional motor feature.
type Feature string

// PositionReporting represesnts the feature of a motor being
// able to report its own position.
const PositionReporting Feature = "PositionReporting"

// ProtoFeaturesToMap takes a MotorServiceGetFeaturesResponse and returns
// an equivalent Feature-to-boolean map.
func ProtoFeaturesToMap(resp *pb.MotorServiceGetFeaturesResponse) map[Feature]bool {
	return map[Feature]bool{
		PositionReporting: resp.PositionReporting,
	}
}

// FeatureMapToProtoResponse takes a map of features to booleans (indicating
// whether the feature is supported) and converts it to a MotorServiceGetFeaturesResponse.
func FeatureMapToProtoResponse(
	featureMap map[Feature]bool,
) (*pb.MotorServiceGetFeaturesResponse, error) {
	return &pb.MotorServiceGetFeaturesResponse{
		PositionReporting: featureMap[PositionReporting],
	}, nil
}
