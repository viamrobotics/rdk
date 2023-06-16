package encoder

import pb "go.viam.com/api/component/encoder/v1"

type Properties struct {
	TicksCountSupported   bool
	AngleDegreesSupported bool
}

// ProtoFeaturesToProperties takes a GetPropertiesResponse and returns
// an equivalent Properties struct.
func ProtoFeaturesToProperties(resp *pb.GetPropertiesResponse) Properties {
	return Properties{
		// A base's truning radius is the minimum radius it can turn around.
		// This can be zero for bases that use differential, omni, mecanum
		// and zero-turn steering bases.
		// Usually non-zero for ackerman, crab and four wheel steered bases
		TicksCountSupported: resp.TicksCountSupported,
		// the width of the base's wheelbase
		AngleDegreesSupported: resp.AngleDegreesSupported,
	}
}

// PropertiesToProtoResponse takes a properties struct and converts it
// to a GetPropertiesResponse.
func PropertiesToProtoResponse(
	features Properties,
) (*pb.GetPropertiesResponse, error) {
	return &pb.GetPropertiesResponse{
		TicksCountSupported:   features.TicksCountSupported,
		AngleDegreesSupported: features.AngleDegreesSupported,
	}, nil
}
