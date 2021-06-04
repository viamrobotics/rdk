// Package camera defines a frame capturing device.
package camera

import (
	"context"

	"github.com/edaniels/gostream"

	"go.viam.com/core/pointcloud"
	"go.viam.com/core/rimage"
)

// A Camera represents anything that can capture frames.
type Camera interface {
	gostream.ImageSource
	NextPointCloud(ctx context.Context) (pointcloud.PointCloud, error)
}

// ImageSource implements a Camera with a gostream.ImageSource.
type ImageSource struct {
	gostream.ImageSource
}

// NextPointCloud returns the next PointCloud from the camera, or will error if not supported
func (is *ImageSource) NextPointCloud(ctx context.Context) (pointcloud.PointCloud, error) {
	img, closer, err := is.Next(ctx)
	if err != nil {
		return nil, err
	}
	defer closer()
	return rimage.ConvertToImageWithDepth(img).ToPointCloud()
}
