package rimage

import (
	"image/color"

	"github.com/go-errors/errors"

	"go.viam.com/core/pointcloud"
)

// A CameraSystem stores the system of camera models, the intrinsic parameters of each camera,
// and the extrinsics that relate them to each other. Used for image alignment and 2D<->3D projection.
type CameraSystem interface {
	AlignImageWithDepth(*ImageWithDepth) (*ImageWithDepth, error)
	ImageWithDepthToPointCloud(*ImageWithDepth) (pointcloud.PointCloud, error)
	PointCloudToImageWithDepth(pointcloud.PointCloud) (*ImageWithDepth, error)
}

// IsAligned returns if the image and depth are aligned.
func (i *ImageWithDepth) IsAligned() bool {
	return i.aligned
}

// CameraSystem returns the camera system that captured the image.
func (i *ImageWithDepth) CameraSystem() CameraSystem {
	return i.camera
}

// SetCameraSystem sets the camera system that captured the image.
func (i *ImageWithDepth) SetCameraSystem(s CameraSystem) {
	i.camera = s
}

// ToPointCloud takes a 2D ImageWithDepth and projects it to a 3D PointCloud. If no CameraSystem
// is available, a default parallel projection is applied, which is most likely unideal.
func (i *ImageWithDepth) ToPointCloud() (pointcloud.PointCloud, error) {
	if i.camera == nil {
		return defaultToPointCloud(i)
	}
	return i.camera.ImageWithDepthToPointCloud(i)
}

// Parallel projections to pointclouds are done in a naive way that don't take any camera parameters into account
func defaultToPointCloud(ii *ImageWithDepth) (pointcloud.PointCloud, error) {
	if !ii.IsAligned() {
		return nil, errors.New("input ImageWithDepth is not aligned")
	}
	pc := pointcloud.New()
	height := ii.Height()
	width := ii.Width()
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			z := ii.Depth.GetDepth(x, y)
			if z == 0 {
				continue
			}
			c := ii.Color.GetXY(x, y)
			r, g, b := c.RGB255()
			err := pc.Set(pointcloud.NewColoredPoint(float64(x), float64(y), float64(z), color.NRGBA{r, g, b, 255}))
			if err != nil {
				return nil, err
			}
		}
	}
	return pc, nil
}
