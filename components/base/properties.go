// Package base contains an enum representing optional base features
package base

import pb "go.viam.com/api/component/base/v1"

// Properties is a structure representing features
// of a base.
type Properties struct {
	TurningRadiusMeters      float64
	WidthMeters              float64
	WheelCircumferenceMeters float64
}

// ProtoFeaturesToProperties takes a GetPropertiesResponse and returns
// an equivalent Properties struct.
func ProtoFeaturesToProperties(resp *pb.GetPropertiesResponse) Properties {
	return Properties{
		// A base's truning radius is the minimum radius it can turn around.
		// This can be zero for bases that use differential, omni, mecanum
		// and zero-turn steering bases.
		// Usually non-zero for ackerman, crab and four wheel steered bases
		TurningRadiusMeters: resp.TurningRadiusMeters,
		// the width of the base's wheelbase
		WidthMeters: resp.WidthMeters,
		// the circumference of the wheels
		WheelCircumferenceMeters: resp.WheelCircumferenceMeters,
	}
}

// PropertiesToProtoResponse takes a map of features to struct and converts it
// to a GetPropertiesResponse.
func PropertiesToProtoResponse(
	features Properties,
) (*pb.GetPropertiesResponse, error) {
	return &pb.GetPropertiesResponse{
		TurningRadiusMeters:      features.TurningRadiusMeters,
		WidthMeters:              features.WidthMeters,
		WheelCircumferenceMeters: features.WheelCircumferenceMeters,
	}, nil
}
