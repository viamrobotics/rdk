package imagesource

import (
	"context"
	"image"

	"github.com/edaniels/golog"
	"github.com/edaniels/gostream"
	"github.com/pkg/errors"

	"go.viam.com/rdk/component/camera"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/pointcloud"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/rimage"
	"go.viam.com/rdk/robot"
	"go.viam.com/rdk/vision/segmentation"
)

func init() {
	registry.RegisterComponent(
		camera.Subtype,
		"color_segments",
		registry.Component{Constructor: func(
			ctx context.Context,
			r robot.Robot,
			config config.Component,
			logger golog.Logger,
		) (interface{}, error) {
			attrs, ok := config.ConvertedAttributes.(*camera.AttrConfig)
			if !ok {
				return nil, errors.Errorf("expected config.ConvertedAttributes to be *camera.AttrConfig but got %T", config.ConvertedAttributes)
			}
			source, ok := r.CameraByName(attrs.Source)
			if !ok {
				return nil, errors.Errorf("cannot find source camera (%s)", attrs.Source)
			}
			return newColorSegmentsSource(source, attrs)
		}})

	config.RegisterComponentAttributeMapConverter(config.ComponentTypeCamera, "color_segments",
		func(attributes config.AttributeMap) (interface{}, error) {
			var conf camera.AttrConfig
			return config.TransformAttributeMapToStruct(&conf, attributes)
		}, &camera.AttrConfig{})
}

// colorSegmentsSource applies a segmentation to the point cloud of an ImageWithDepth.
type colorSegmentsSource struct {
	source gostream.ImageSource
	config segmentation.ObjectConfig
	proj   rimage.Projector
}

// Next applies segmentation to the next image and gives each distinct object a unique color.
func (cs *colorSegmentsSource) Next(ctx context.Context) (image.Image, func(), error) {
	i, closer, err := cs.source.Next(ctx)
	if err != nil {
		return i, closer, err
	}
	defer closer()
	ii := rimage.ConvertToImageWithDepth(i)
	if ii.Depth == nil {
		return nil, nil, errors.New("colorSegmentsSource Next(): no depth")
	}
	if cs.proj == nil {
		return nil, nil, errors.New("colorSegmentsSource Next(): no projector")
	}
	cloud, err := cs.proj.ImageWithDepthToPointCloud(ii)
	if err != nil {
		return nil, nil, err
	}
	segments, err := segmentation.NewObjectSegmentation(ctx, cloud, cs.config)
	if err != nil {
		return nil, nil, err
	}
	colorCloud, err := pointcloud.MergePointCloudsWithColor(segments.PointClouds())
	if err != nil {
		return nil, nil, err
	}
	segmentedIwd, err := cs.proj.PointCloudToImageWithDepth(colorCloud)
	if err != nil {
		return nil, nil, err
	}
	return segmentedIwd, func() {}, nil
}

func newColorSegmentsSource(source camera.Camera, attrs *camera.AttrConfig) (camera.Camera, error) {
	planeSize := attrs.PlaneSize
	if attrs.PlaneSize == 0 {
		attrs.PlaneSize = 10000
	}
	segmentSize := attrs.SegmentSize
	if attrs.SegmentSize == 0 {
		attrs.SegmentSize = 5
	}
	clusterRadius := attrs.ClusterRadius
	if attrs.ClusterRadius == 0 {
		attrs.ClusterRadius = 5.0
	}
	cfg := segmentation.ObjectConfig{
		MinPtsInPlane: planeSize, MinPtsInSegment: segmentSize, ClusteringRadiusMm: clusterRadius,
	}
	segSrc := &colorSegmentsSource{source, cfg, attrs.CameraParameters}
	return camera.New(segSrc, attrs, source)
}
