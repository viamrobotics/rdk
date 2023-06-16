package movementsensor

import pb "go.viam.com/api/component/movementsensor/v1"

// Properties is a structure representing features
// of a movementsensor.
type Properties struct {
	LinearVelocitySupported     bool
	AngularVelocitySupported    bool
	OrientationSupported        bool
	PositionSupported           bool
	CompassHeadingSupported     bool
	LinearAccelerationSupported bool
}

// ProtoFeaturesToProperties takes a GetPropertiesResponse and returns
// an equivalent Properties struct.
func ProtoFeaturesToProperties(resp *pb.GetPropertiesResponse) *Properties {
	return &Properties{
		LinearVelocitySupported:     resp.LinearVelocitySupported,
		AngularVelocitySupported:    resp.AngularVelocitySupported,
		OrientationSupported:        resp.OrientationSupported,
		PositionSupported:           resp.PositionSupported,
		CompassHeadingSupported:     resp.CompassHeadingSupported,
		LinearAccelerationSupported: resp.LinearAccelerationSupported,
	}
}

// PropertiesToProtoResponse takes a properties struct and converts it
// to a GetPropertiesResponse.
func PropertiesToProtoResponse(
	features *Properties,
) (*pb.GetPropertiesResponse, error) {
	return &pb.GetPropertiesResponse{
		LinearVelocitySupported:     features.LinearVelocitySupported,
		AngularVelocitySupported:    features.AngularVelocitySupported,
		OrientationSupported:        features.OrientationSupported,
		PositionSupported:           features.PositionSupported,
		CompassHeadingSupported:     features.CompassHeadingSupported,
		LinearAccelerationSupported: features.LinearAccelerationSupported,
	}, nil
}
