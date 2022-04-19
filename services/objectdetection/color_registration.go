package objectdetection

import (
	"context"
	"errors"

	"go.viam.com/rdk/config"
	"go.viam.com/rdk/utils"
	objdet "go.viam.com/rdk/vision/objectdetection"
)

// registerColorDetector parses the Parameter field from the registry config into ColorDetectorConfig,
// creates the ColorDetector, and registers it to the detector registry.
func registerColorDetector(ctx context.Context, r detRegistry, conf *RegistryConfig) error {
	if conf == nil {
		return errors.New("object detection registry config cannot be nil")
	}
	var p objdet.ColorDetectorConfig
	attrs, err := config.TransformAttributeMapToStruct(&p, conf.Parameters)
	if err != nil {
		return err
	}
	params, ok := attrs.(*objdet.ColorDetectorConfig)
	if !ok {
		return utils.NewUnexpectedTypeError(params, attrs)
	}
	detector, err := objdet.NewColorDetector(params)
	if err != nil {
		return err
	}
	return r.RegisterDetector(ctx, conf.Name, detector)
}
