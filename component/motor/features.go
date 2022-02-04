// Package motor contains an enum representing optional motor features
package motor

import (
	pb "go.viam.com/rdk/proto/api/component/v1"
)

// Feature is an enum representing an optional motor feature.
type Feature int

// PositionReporting represesnts the feature of a motor being
// able to report its own position.
const PositionReporting Feature = iota

func (feature Feature) String() string {
	if feature == PositionReporting {
		return "position_reporting"
	}
	return "unknown_feature"
}

type setter func(resp *pb.MotorServiceGetFeaturesResponse, isSupported bool)

type reader func(resp *pb.MotorServiceGetFeaturesResponse) bool

// FeatureToResponseSetter is a mapping of motor feature to
// the name of the corresponding key in the gRPC response to GetFeatures.
var FeatureToResponseSetter = map[Feature]setter{
	PositionReporting: func(resp *pb.MotorServiceGetFeaturesResponse, isSupported bool) {
		resp.PositionReporting = isSupported
	},
}

// FeatureToResponseReader is a mapping of motor feature to
// the name of the corresponding key in the gRPC response to GetFeatures.
var FeatureToResponseReader = map[Feature]reader{
	PositionReporting: func(resp *pb.MotorServiceGetFeaturesResponse) bool {
		return resp.PositionReporting
	},
}
