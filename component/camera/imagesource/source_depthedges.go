package imagesource

import (
	"context"
	"image"

	"github.com/edaniels/golog"
	"github.com/edaniels/gostream"
	"github.com/pkg/errors"

	"go.viam.com/rdk/component/camera"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/rimage"
	"go.viam.com/rdk/robot"
)

func init() {
	registry.RegisterComponent(
		camera.Subtype,
		"depth_edges",
		registry.Component{Constructor: func(
			ctx context.Context,
			r robot.Robot,
			config config.Component,
			logger golog.Logger,
		) (interface{}, error) {
			return newDepthEdgesSource(r, config)
		}})

	config.RegisterComponentAttributeMapConverter(config.ComponentTypeCamera, "depth_edges",
		func(attributes config.AttributeMap) (interface{}, error) {
			var conf rimage.AttrConfig
			return config.TransformAttributeMapToStruct(&conf, attributes)
		}, &rimage.AttrConfig{})
}

// depthEdgesSource applies a Canny Edge Detector to the depth map of the ImageWithDepth.
type depthEdgesSource struct {
	source     gostream.ImageSource
	detector   *rimage.CannyEdgeDetector
	blurRadius float64
}

// Next applies a canny edge detector on the depth map of the next image.
func (os *depthEdgesSource) Next(ctx context.Context) (image.Image, func(), error) {
	i, closer, err := os.source.Next(ctx)
	if err != nil {
		return i, closer, err
	}
	defer closer()
	ii := rimage.ConvertToImageWithDepth(i)
	if ii.Depth == nil {
		return nil, nil, errors.New("no depth")
	}
	edges, err := os.detector.DetectDepthEdges(ii.Depth, os.blurRadius)
	if err != nil {
		return nil, nil, err
	}
	return edges, func() {}, nil
}

func newDepthEdgesSource(r robot.Robot, config config.Component) (camera.Camera, error) {
	source, ok := r.CameraByName(config.Attributes.String("source"))
	if !ok {
		return nil, errors.Errorf("cannot find source camera (%s)", config.Attributes.String("source"))
	}
	canny := rimage.NewCannyDericheEdgeDetectorWithParameters(0.85, 0.40, true)
	return &camera.ImageSource{ImageSource: &depthEdgesSource{source, canny, 3.0}}, nil
}
