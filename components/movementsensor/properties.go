package movementsensor

import pb "go.viam.com/api/component/movementsensor/v1"

// Properties is a structure representing features
// of a movementsensor.
type Properties struct {
	PositionSupported           bool
	LinearVelocitySupported     bool
	AngularVelocitySupported    bool
	LinearAccelerationSupported bool
	CompassHeadingSupported     bool
	OrientationSupported        bool
}

// ProtoFeaturesToProperties takes a GetPropertiesResponse and returns
// an equivalent Properties struct.
func ProtoFeaturesToProperties(resp *pb.GetPropertiesResponse) *Properties {
	return &Properties{
		PositionSupported:           resp.PositionSupported,
		LinearVelocitySupported:     resp.LinearVelocitySupported,
		AngularVelocitySupported:    resp.AngularVelocitySupported,
		LinearAccelerationSupported: resp.LinearAccelerationSupported,
		CompassHeadingSupported:     resp.CompassHeadingSupported,
		OrientationSupported:        resp.OrientationSupported,
	}
}

// PropertiesToProtoResponse takes a properties struct and converts it
// to a GetPropertiesResponse.
func PropertiesToProtoResponse(
	features *Properties,
) (*pb.GetPropertiesResponse, error) {
	return &pb.GetPropertiesResponse{
		PositionSupported:           features.PositionSupported,
		LinearVelocitySupported:     features.LinearVelocitySupported,
		AngularVelocitySupported:    features.AngularVelocitySupported,
		LinearAccelerationSupported: features.LinearAccelerationSupported,
		CompassHeadingSupported:     features.CompassHeadingSupported,
		OrientationSupported:        features.OrientationSupported,
	}, nil
}
