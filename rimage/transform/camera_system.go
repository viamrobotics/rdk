//go:build !notc

package transform

import (
	"image"

	"github.com/golang/geo/r3"

	"go.viam.com/rdk/pointcloud"
	"go.viam.com/rdk/rimage"
)

// Aligner aligns a color and depth image together.
type Aligner interface {
	AlignColorAndDepthImage(*rimage.Image, *rimage.DepthMap) (*rimage.Image, *rimage.DepthMap, error)
}

// Projector can transform a scene between a 2D Image and DepthMap and a 3D pointcloud.
type Projector interface {
	// Project a 2D RGBD image to 3D pointcloud. Can add an optional crop to the image before projection.
	RGBDToPointCloud(*rimage.Image, *rimage.DepthMap, ...image.Rectangle) (pointcloud.PointCloud, error)
	// Project a 3D pointcloud to a 2D RGBD image.
	PointCloudToRGBD(pointcloud.PointCloud) (*rimage.Image, *rimage.DepthMap, error)
	// Project a single pixel point to a given depth.
	ImagePointTo3DPoint(image.Point, rimage.Depth) (r3.Vector, error)
}

// A CameraSystem stores the system of camera models, the intrinsic parameters of each camera,
// and the distortion model.
type CameraSystem interface {
	Projector
	Distorter
}
