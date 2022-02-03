// Package motor contains an enum representing optional motor features
package motor

import (
	pb "go.viam.com/rdk/proto/api/component/v1"
)

// MotorFeature is an enum representing an optional motor feature
type MotorFeature int

// PositionReporting represesnts the feature of a motor being
// able to report its own position
const PositionReporting MotorFeature = iota

func (mf MotorFeature) String() string {
	switch mf {
	case PositionReporting:
		return "position_reporting"
	}
	return "unknown_feature"
}

type setter func(resp *pb.MotorServiceGetFeaturesResponse, isSupported bool)

type reader func(resp *pb.MotorServiceGetFeaturesResponse) bool

// FeatureToResponseSetter is a mapping of motor feature to
// the name of the corresponding key in the gRPC response to GetFeatures
var FeatureToResponseSetter = map[MotorFeature]setter{
	PositionReporting: func(resp *pb.MotorServiceGetFeaturesResponse, isSupported bool) {
		resp.PositionReporting = isSupported
	},
}

// FeatureToResponseReader is a mapping of motor feature to
// the name of the corresponding key in the gRPC response to GetFeatures
var FeatureToResponseReader = map[MotorFeature]reader{
	PositionReporting: func(resp *pb.MotorServiceGetFeaturesResponse) bool {
		return resp.PositionReporting
	},
}
