package rimage

import (
	"image"

	"github.com/golang/geo/r3"

	"go.viam.com/rdk/pointcloud"
)

// Aligner aligns a color and depth image together.
type Aligner interface {
	AlignImageWithDepth(*ImageWithDepth) (*ImageWithDepth, error)
}

// Projector can transform a scene between a 2D ImageWithDepth and a 3D pointcloud.
type Projector interface {
	ImageWithDepthToPointCloud(*ImageWithDepth) (pointcloud.PointCloud, error)
	PointCloudToImageWithDepth(pointcloud.PointCloud) (*ImageWithDepth, error)
	ImagePointTo3DPoint(image.Point, Depth) (r3.Vector, error)
}

// A CameraSystem stores the system of camera models, the intrinsic parameters of each camera,
// and the extrinsics that relate them to each other. Used for image alignment and 2D<->3D projection.
type CameraSystem interface {
	Aligner
	Projector
}
