package camera

import (
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/rimage/transform"
	"go.viam.com/rdk/utils"
)

// AttrConfig is exported to be used as an attribute map for settings common to all camera types.
type AttrConfig struct {
	Source           string                             `json:"source"`
	Debug            bool                               `json:"debug"`
	Width            int                                `json:"width"`
	Height           int                                `json:"height"`
	Dump             bool                               `json:"dump"`
	CameraParameters *transform.PinholeCameraIntrinsics `json:"camera_parameters"`
}

// CommonCameraAttributes extracts the common camera attributes.
func CommonCameraAttributes(attributes config.AttributeMap) (*AttrConfig, error) {
	var cameraAttrs AttrConfig
	attrs, err := config.TransformAttributeMapToStruct(&cameraAttrs, attributes)
	if err != nil {
		return nil, err
	}
	result, ok := attrs.(*AttrConfig)
	if !ok {
		return nil, utils.NewUnexpectedTypeError(result, attrs)
	}
	return result, nil
}
