package imagesource

import (
	"context"
	"image"

	"github.com/go-errors/errors"

	"github.com/edaniels/golog"
	"github.com/edaniels/gostream"

	"go.viam.com/core/camera"
	"go.viam.com/core/config"
	"go.viam.com/core/pointcloud"
	"go.viam.com/core/registry"
	"go.viam.com/core/rimage"
	"go.viam.com/core/robot"
	"go.viam.com/core/vision/segmentation"
)

func init() {
	registry.RegisterCamera("colorSegments", func(ctx context.Context, r robot.Robot, config config.Component, logger golog.Logger) (camera.Camera, error) {
		return newColorSegmentsSource(r, config)
	})
}

// ColorSegmentsSource applies a segmentation to the point cloud of an ImageWithDepth
type ColorSegmentsSource struct {
	source gostream.ImageSource
	config segmentation.ObjectConfig
}

// Close closes the source
func (cs *ColorSegmentsSource) Close() error {
	return nil
}

// Next applies segmentation to the next image and gives each distinct object a unique color
func (cs *ColorSegmentsSource) Next(ctx context.Context) (image.Image, func(), error) {
	i, closer, err := cs.source.Next(ctx)
	if err != nil {
		return i, closer, err
	}
	defer closer()
	ii := rimage.ConvertToImageWithDepth(i)
	if ii.Depth == nil {
		return nil, nil, errors.New("no depth")
	}
	if ii.CameraSystem() == nil {
		return nil, nil, errors.New("no camera system")
	}
	cloud, err := ii.ToPointCloud()
	if err != nil {
		return nil, nil, err
	}
	segments, err := segmentation.NewObjectSegmentation(cloud, cs.config)
	if err != nil {
		return nil, nil, err
	}
	colorCloud, err := pointcloud.MergePointCloudsWithColor(segments.PointClouds())
	if err != nil {
		return nil, nil, err
	}
	segmentedIwd, err := ii.CameraSystem().PointCloudToImageWithDepth(colorCloud)
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
	cfg := segmentation.ObjectConfig{planeSize, segmentSize, clusterRadius}
	return &camera.ImageSource{&ColorSegmentsSource{source, cfg}}, nil

}
