// Package base contains an enum representing optional base features
package base

import pb "go.viam.com/api/component/base/v1"

// Feature is an structure representing optional base features
// of a base reporting its respective minimum turning radius and
// width respectively, in units of meters (M).
type Feature struct {
	TurningRadiusMeters float64
	WidthMeters         float64
}

// ProtoFeaturesToMap takes a GetPropertiesResponse and returns
// an equivalent Feature-to-boolean map.
func ProtoFeaturesToMap(resp *pb.GetPropertiesResponse) Feature {
	return Feature{
		// A base's truning radius is the minimu radius it can turn around, this can be 
		// zero for bases that use differential, omni, mecanum and zero-turn steering bases,
		// usually non-zero for ackerman, crab and four wheel steered bases
		TurningRadiusMeters: resp.TurningRadiusMeters, 
		// the width of the base's wheelbase
		WidthMeters:         resp.WidthMeters,
	}
}

// FeatureMapToProtoResponse takes a map of features to struct and converts it
// to a GetPropertiesResponse.
func FeatureMapToProtoResponse(
	features Feature,
) (*pb.GetPropertiesResponse, error) {
	return &pb.GetPropertiesResponse{
		TurningRadiusMeters: features.TurningRadiusMeters,
		WidthMeters:         features.WidthMeters,
	}, nil
}
