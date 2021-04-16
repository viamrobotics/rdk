package calib

import (
	"go.viam.com/robotcore/pointcloud"
	"go.viam.com/robotcore/rimage"
)

type DepthColorAligner interface {
	ToAlignedImageWithDepth(*rimage.ImageWithDepth) (*rimage.ImageWithDepth, error)
	ToPointCloudWithColor(*rimage.ImageWithDepth) (*pointcloud.PointCloud, error)
}
