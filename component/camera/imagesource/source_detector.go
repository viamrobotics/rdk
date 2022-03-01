package imagesource

import (
	"context"
	"fmt"

	"github.com/edaniels/golog"

	"go.viam.com/rdk/component/camera"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/rimage"
	"go.viam.com/rdk/robot"
	"go.viam.com/rdk/utils"
	"go.viam.com/rdk/vision/objectdetection"
)

func init() {
	registry.RegisterComponent(
		camera.Subtype,
		"color_detector",
		registry.Component{Constructor: func(
			ctx context.Context,
			r robot.Robot,
			config config.Component,
			logger golog.Logger,
		) (interface{}, error) {
			attrs, ok := config.ConvertedAttributes.(*camera.AttrConfig)
			if !ok {
				return nil, utils.NewUnexpectedTypeError(attrs, config.ConvertedAttributes)
			}
			sourceName := attrs.Source
			src, err := camera.FromRobot(r, sourceName)
			if err != nil {
				return nil, fmt.Errorf("no source camera (%s): %w", sourceName, err)
			}
			return newColorDetector(src, attrs)
		}})

	config.RegisterComponentAttributeMapConverter(
		config.ComponentTypeCamera,
		"color_detector",
		func(attributes config.AttributeMap) (interface{}, error) {
			var conf camera.AttrConfig
			return config.TransformAttributeMapToStruct(&conf, attributes)
		},
		camera.AttrConfig{},
	)
}

// newColorDetector creates a simple color detector from a source camera component in the config and user defined attributes.
func newColorDetector(src camera.Camera, attrs *camera.AttrConfig) (camera.Camera, error) {
	// define the preprocessor
	pSlice := make([]objectdetection.Preprocessor, 0, 3)
	for _, c := range attrs.ExcludeColors {
		rc, err := objectdetection.RemoveColorChannel(c)
		if err != nil {
			return nil, err
		}
		pSlice = append(pSlice, rc)
	}
	p := objectdetection.ComposePreprocessors(pSlice)

	// define the detector
	tolerance := 0.05 // default value of 5%
	if attrs.Tolerance != 0. {
		tolerance = attrs.Tolerance
	}
	col := rimage.Pink // default value
	detectColor, err := attrs.DetectColor()
	if err != nil {
		return nil, err
	}
	if len(detectColor) != 0 {
		col = rimage.NewColor(detectColor[0], detectColor[1], detectColor[2])
	}
	hue, _, _ := col.HsvNormal()
	d, err := objectdetection.NewColorDetector(tolerance, hue)
	if err != nil {
		return nil, err
	}

	// define the filter
	segmentSize := 5000 // default value
	if attrs.SegmentSize != 0 {
		segmentSize = attrs.SegmentSize
	}
	f := objectdetection.NewAreaFilter(segmentSize)

	det, err := objectdetection.Build(p, d, f)
	if err != nil {
		return nil, err
	}
	detector, err := objectdetection.NewSource(src, det)
	if err != nil {
		return nil, err
	}
	return camera.New(detector, attrs, src)
}
