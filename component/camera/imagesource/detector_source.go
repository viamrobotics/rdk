package imagesource

import (
	"context"

	"github.com/edaniels/golog"
	"github.com/edaniels/gostream"
	"github.com/pkg/errors"

	"go.viam.com/rdk/component/camera"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/rimage"
	"go.viam.com/rdk/robot"
	"go.viam.com/rdk/vision/objectdetection"
)

func init() {
	registry.RegisterComponent(
		camera.Subtype,
		"simple_detector",
		registry.Component{Constructor: func(
			ctx context.Context,
			r robot.Robot,
			config config.Component,
			logger golog.Logger,
		) (interface{}, error) {
			attrs, ok := config.ConvertedAttributes.(*rimage.AttrConfig)
			if !ok {
				return nil, errors.New("cannot convert config.ConvertedAttributes into a *rimage.AttrConfig")
			}
			sourceName := attrs.Source
			source, ok := r.CameraByName(sourceName)
			if !ok {
				return nil, errors.Errorf("cannot find source camera (%s)", sourceName)
			}
			return NewColorDetector(source, attrs)
		}})

	config.RegisterComponentAttributeMapConverter(
		config.ComponentTypeCamera,
		"simple_detector",
		func(attributes config.AttributeMap) (interface{}, error) {
			var conf rimage.AttrConfig
			return config.TransformAttributeMapToStruct(&conf, attributes)
		},
		rimage.AttrConfig{},
	)
}

// NewColorDetector creates a simple color detector from a source camera component in the config and user defined attributes.
func NewColorDetector(src gostream.ImageSource, attrs *rimage.AttrConfig) (*camera.ImageSource, error) {
	// define the preprocessor
	pSlice := make([]objectdetection.Preprocessor, 0, 3)
	if len(attrs.ExcludeColors) != 0 {
		for _, c := range attrs.ExcludeColors {
			rc, err := objectdetection.RemoveColorChannel(c)
			if err != nil {
				return nil, err
			}
			pSlice = append(pSlice, rc)
		}
	}
	p := objectdetection.ComposePreprocessors(pSlice)

	// define the detector
	tolerance := 10.0 // default value
	if attrs.Tolerance != 0. {
		tolerance = attrs.Tolerance
	}
	col := rimage.Pink // default value
	if len(attrs.DetectColor) != 0 {
		if len(attrs.DetectColor) != 3 {
			return nil, errors.Errorf("detect_color must be list of ints in format [r, g, b], got %v", attrs.DetectColor)
		} else {
			col = rimage.NewColor(attrs.DetectColor[0], attrs.DetectColor[1], attrs.DetectColor[2])
		}
	}
	d := objectdetection.NewColorDetector(tolerance, col)

	// define the filter
	segmentSize := 5000 // default value
	if attrs.SegmentSize != 0 {
		segmentSize = attrs.SegmentSize
	}
	f := objectdetection.NewAreaFilter(segmentSize)

	detector, err := objectdetection.NewSource(src, p, d, f)
	if err != nil {
		return nil, err
	}
	return &camera.ImageSource{ImageSource: detector}, nil
}
