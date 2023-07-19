package encoder

import pb "go.viam.com/api/component/encoder/v1"

// Properties holds the properties of the encoder.
type Properties struct {
	TicksCountSupported   bool
	AngleDegreesSupported bool
}

// ProtoFeaturesToProperties takes a GetPropertiesResponse and returns
// an equivalent Properties struct.
func ProtoFeaturesToProperties(resp *pb.GetPropertiesResponse) Properties {
	return Properties{
		TicksCountSupported:   resp.TicksCountSupported,
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
