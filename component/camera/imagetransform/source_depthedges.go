package imagetransform

import (
	"context"
	"fmt"
	"image"

	"github.com/edaniels/golog"
	"github.com/edaniels/gostream"

	"go.viam.com/rdk/component/camera"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/rimage"
	"go.viam.com/rdk/utils"
)

func init() {
	registry.RegisterComponent(
		camera.Subtype,
		"depth_edges",
		registry.Component{Constructor: func(
			ctx context.Context,
			deps registry.Dependencies,
			config config.Component,
			logger golog.Logger,
		) (interface{}, error) {
			attrs, ok := config.ConvertedAttributes.(*camera.AttrConfig)
			if !ok {
				return nil, utils.NewUnexpectedTypeError(attrs, config.ConvertedAttributes)
			}
			return newDepthEdgesSource(ctx, deps, attrs)
		}})

	config.RegisterComponentAttributeMapConverter(camera.SubtypeName, "depth_edges",
		func(attributes config.AttributeMap) (interface{}, error) {
			var conf camera.AttrConfig
			return config.TransformAttributeMapToStruct(&conf, attributes)
		}, &camera.AttrConfig{})
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
	dm, err := rimage.ConvertImageToDepthMap(i)
	if err != nil {
		return nil, nil, err
	}
	edges, err := os.detector.DetectDepthEdges(dm, os.blurRadius)
	if err != nil {
		return nil, nil, err
	}
	return edges, func() {}, nil
}

func newDepthEdgesSource(ctx context.Context, deps registry.Dependencies, attrs *camera.AttrConfig) (camera.Camera, error) {
	source, err := camera.FromDependencies(deps, attrs.Source)
	if err != nil {
		return nil, fmt.Errorf("no source camera (%s): %w", attrs.Source, err)
	}
	canny := rimage.NewCannyDericheEdgeDetectorWithParameters(0.85, 0.40, true)
	imgSrc := &depthEdgesSource{source, canny, 3.0}
	return camera.New(imgSrc, camera.GetProjector(ctx, attrs, source))
}
