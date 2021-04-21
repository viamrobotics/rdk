package calib

import (
	"go.viam.com/robotcore/pointcloud"
	"go.viam.com/robotcore/rimage"
)

type DepthColorAligner interface {
	AlignImageWithDepth(*rimage.ImageWithDepth) (*rimage.ImageWithDepth, error)
	ImageWithDepthToPointCloud(*rimage.ImageWithDepth) (*pointcloud.PointCloud, error)
	PointCloudToImageWithDepth(*pointcloud.PointCloud) (*rimage.ImageWithDepth, error)
}
