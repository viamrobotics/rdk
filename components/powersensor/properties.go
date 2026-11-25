package powersensor

import pb "go.viam.com/api/component/powersensor/v1"

// Properties is a structure representing features of a power sensor.
type Properties struct {
	IsBatteryPercentSupported bool
	IsChargingSupported       bool
}

// ProtoFeaturesToProperties takes a GetPropertiesResponse and returns
// an equivalent Properties struct.
func ProtoFeaturesToProperties(resp *pb.GetPropertiesResponse) *Properties {
	return &Properties{
		IsBatteryPercentSupported: resp.IsBatteryPercentSupported,
		IsChargingSupported:       resp.IsChargingSupported,
	}
}

// PropertiesToProtoResponse takes a Properties struct and converts it
// to a GetPropertiesResponse.
func PropertiesToProtoResponse(props *Properties) *pb.GetPropertiesResponse {
	return &pb.GetPropertiesResponse{
		IsBatteryPercentSupported: props.IsBatteryPercentSupported,
		IsChargingSupported:       props.IsChargingSupported,
	}
}
