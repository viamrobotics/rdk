package movementsensor

import pb "go.viam.com/api/component/movementsensor/v1"

// Properties is a structure representing features
// of a movementsensor.
// The order is in terms of order of derivatives in time
// with position, orientation, compassheading at the top (zeroth derivative)
// linear and angular velocities next (first derivative)
// linear acceleration laste (second derivative).
type Properties struct {
	PositionSupported           bool
	OrientationSupported        bool
	CompassHeadingSupported     bool
	LinearVelocitySupported     bool
	AngularVelocitySupported    bool
	LinearAccelerationSupported bool
}

// ProtoFeaturesToProperties takes a GetPropertiesResponse and returns
// an equivalent Properties struct.
func ProtoFeaturesToProperties(resp *pb.GetPropertiesResponse) *Properties {
	return &Properties{
		PositionSupported:           resp.PositionSupported,
		OrientationSupported:        resp.OrientationSupported,
		CompassHeadingSupported:     resp.CompassHeadingSupported,
		LinearVelocitySupported:     resp.LinearVelocitySupported,
		AngularVelocitySupported:    resp.AngularVelocitySupported,
		LinearAccelerationSupported: resp.LinearAccelerationSupported,
	}
}

// PropertiesToProtoResponse takes a properties struct and converts it
// to a GetPropertiesResponse.
func PropertiesToProtoResponse(
	features *Properties,
) (*pb.GetPropertiesResponse, error) {
	return &pb.GetPropertiesResponse{
		PositionSupported:           features.PositionSupported,
		OrientationSupported:        features.OrientationSupported,
		CompassHeadingSupported:     features.CompassHeadingSupported,
		LinearVelocitySupported:     features.LinearVelocitySupported,
		AngularVelocitySupported:    features.AngularVelocitySupported,
		LinearAccelerationSupported: features.LinearAccelerationSupported,
	}, nil
}
