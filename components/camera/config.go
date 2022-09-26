package camera

import (
	"github.com/pkg/errors"

	"go.viam.com/rdk/config"
	"go.viam.com/rdk/rimage/transform"
	"go.viam.com/rdk/utils"
)

// StreamType specifies what kind of image stream is coming from the camera.
type StreamType string

// The allowed types of streams that can come from a VideoSource.
const (
	UnspecifiedStream = StreamType("")
	ColorStream       = StreamType("color")
	DepthStream       = StreamType("depth")
)

// NewUnsupportedStreamError is when the stream type is unknown.
func NewUnsupportedStreamError(s StreamType) error {
	return errors.Errorf("stream of type %q not supported", s)
}

// AttrConfig is exported to be used as an attribute map for settings common to all camera types.
type AttrConfig struct {
	CameraParameters     *transform.PinholeCameraIntrinsics `json:"intrinsic_parameters"`
	DistortionParameters *transform.BrownConrady            `json:"distortion_parameters"`
	Stream               string                             `json:"stream"`
	Debug                bool                               `json:"debug"`
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
