// Package motor contains an enum representing optional motor features
package motor

import (
	"github.com/pkg/errors"

	pb "go.viam.com/rdk/proto/api/component/v1"
)

// Feature is an enum representing an optional motor feature.
type Feature string

// PositionReporting represesnts the feature of a motor being
// able to report its own position.
const PositionReporting Feature = "PositionReporting"

// NewFeatureUnsupportedError returns an error representing the need
// for a motor to support a particular feature
func NewFeatureUnsupportedError(feature Feature, motorName string) error {
	return errors.Errorf("motor named %s must support feature motor.%s", motorName, feature)
}

// NewUnexpectedFeatureError returns an error particular to when a
// motor Feature is not properly handled by setFeatureBoolean.
func NewUnexpectedFeatureError(feature Feature) error {
	return errors.Errorf("%s is not a handled or expected feature supported by motor", feature)
}

// setFeatureBoolean converts a feature-boolean pair in a GetFeatures result
// to the required flag in a protobuf response.
func setFeatureBoolean(
	feature Feature, isSupported bool,
	resp *pb.MotorServiceGetFeaturesResponse,
) error {
	if feature == PositionReporting {
		resp.PositionReporting = isSupported
	} else {
		return errors.New("unexpected feature")
	}
	return nil
}

// ProtoFeaturesToMap takes a MotorServiceGetFeaturesResponse and returns
// an equivalent Feature-to-boolean map.
func ProtoFeaturesToMap(resp *pb.MotorServiceGetFeaturesResponse) map[Feature]bool {
	return map[Feature]bool{
		PositionReporting: resp.PositionReporting,
	}
}

// FeatureMapToProtoResponse takes a map of features to booleans (indicating
// whether the feature is supported) and converts it to a MotorServiceGetFeaturesResponse.
func FeatureMapToProtoResponse(
	featureMap map[Feature]bool,
) (*pb.MotorServiceGetFeaturesResponse, error) {
	result := &pb.MotorServiceGetFeaturesResponse{}
	for feature, isSupported := range featureMap {
		err := setFeatureBoolean(feature, isSupported, result)
		if err != nil {
			return nil, err
		}
	}
	return result, nil
}
