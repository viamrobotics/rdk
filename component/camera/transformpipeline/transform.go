package transformpipeline

import (
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/utils"
)

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
