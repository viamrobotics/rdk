package arm

import (
	pb "go.viam.com/api/component/arm/v1"
)

// Properties is a struct containing the optional arm properties.
type Properties struct {
	SupportManualMode        bool
	SupportCartesianCommands bool
}

// ProtoFeaturesToProperties takes a GetPropertiesResponse and returns
// an equivalent Properties struct.
func ProtoFeaturesToProperties(resp *pb.GetPropertiesResponse) Properties {
	return Properties{
		SupportManualMode:        resp.SupportManualMode,
		SupportCartesianCommands: resp.SupportCartesianCommands,
	}
}

// PropertiesToProtoResponse takes a Properties struct (indicating
// whether the property is supported) and converts it to a GetPropertiesResponse.
func PropertiesToProtoResponse(
	props Properties,
) (*pb.GetPropertiesResponse, error) {
	return &pb.GetPropertiesResponse{
		SupportManualMode:        props.SupportManualMode,
		SupportCartesianCommands: props.SupportCartesianCommands,
	}, nil
}
