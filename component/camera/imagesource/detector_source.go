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
			return NewSimpleObjectDetector(source, attrs)
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

// NewSimpleObjectDetector creates a simple detector from a source camera component in the config and user defined attributes.
func NewSimpleObjectDetector(src gostream.ImageSource, attrs *rimage.AttrConfig) (*objectdetection.Source, error) {
	threshold := 10.0 // default value
	if attrs.Threshold != 0. {
		threshold = attrs.Threshold
	}
	segmentSize := 5000 // default value
	if attrs.SegmentSize != 0 {
		segmentSize = attrs.SegmentSize
	}
	fps := 33. // default value
	if attrs.Fps != 0 {
		fps = attrs.Fps
	}
	p, err := objectdetection.RemoveColorChannel("b")
	if err != nil {
		return nil, err
	}
	d := objectdetection.NewSimpleDetector(threshold)
	f := objectdetection.NewAreaFilter(segmentSize)
	return objectdetection.NewSource(src, p, d, f, fps)
}
