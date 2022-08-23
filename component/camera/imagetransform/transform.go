package imagetransform

import (
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/utils"
)

// transformConfig is exported to be used as an attribute map for settings common to all transforms.
type transformConfig struct {
	Source string `json:"source"`
	Stream string `json:"stream"`
	Debug  bool   `json:"debug"`
	Height int    `json:"height"`
	Width  int    `json:"width"`
}

// extractAttributes extracts the common transform attributes.
func extractAttributes(attributes config.AttributeMap) (*transformConfig, error) {
	var transformAttrs transformConfig
	attrs, err := config.TransformAttributeMapToStruct(&transformAttrs, attributes)
	if err != nil {
		return nil, err
	}
	result, ok := attrs.(*transformConfig)
	if !ok {
		return nil, utils.NewUnexpectedTypeError(result, attrs)
	}
	return result, nil
}
