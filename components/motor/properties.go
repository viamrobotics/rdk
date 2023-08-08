// Package motor contains a struct representing optional motor properties
package motor

import (
	pb "go.viam.com/api/component/motor/v1"
)

// Properties is struct contaning the motor properties.
type Properties struct {
	PositionReporting bool
}

// ProtoFeaturesToProperties takes a GetPropertiesResponse and returns
// an equivalent Properties struct.
func ProtoFeaturesToProperties(resp *pb.GetPropertiesResponse) Properties {
	return Properties{
		PositionReporting: resp.PositionReporting,
	}
}

// PropertiesToProtoResponse takes a Properties struct (indicating
// whether the property is supported) and converts it to a GetPropertiesResponse.
func PropertiesToProtoResponse(
	props Properties,
) (*pb.GetPropertiesResponse, error) {
	return &pb.GetPropertiesResponse{
		PositionReporting: props.PositionReporting,
	}, nil
}
