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
			return newColorSegmentsSource(r, config)
		}})
}

// colorSegmentsSource applies a segmentation to the point cloud of an ImageWithDepth.
type colorSegmentsSource struct {
	source gostream.ImageSource
	config segmentation.ObjectConfig
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
		return nil, nil, errors.New("no depth")
	}
	if ii.Projector() == nil {
		return nil, nil, errors.New("no camera system")
	}
	cloud, err := ii.ToPointCloud()
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
	segmentedIwd, err := ii.Projector().PointCloudToImageWithDepth(colorCloud)
	if err != nil {
		return nil, nil, err
	}
	return segmentedIwd, func() {}, nil
}

func newColorSegmentsSource(r robot.Robot, config config.Component) (camera.Camera, error) {
	source, ok := r.CameraByName(config.Attributes.String("source"))
	if !ok {
		return nil, errors.Errorf("cannot find source camera (%s)", config.Attributes.String("source"))
	}
	planeSize := config.Attributes.Int("plane_size", 10000)
	segmentSize := config.Attributes.Int("segment_size", 5)
	clusterRadius := config.Attributes.Float64("cluster_radius", 5.0)
	cfg := segmentation.ObjectConfig{
		MinPtsInPlane: planeSize, MinPtsInSegment: segmentSize, ClusteringRadius: clusterRadius,
	}
	return &camera.ImageSource{ImageSource: &colorSegmentsSource{source, cfg}}, nil
}
