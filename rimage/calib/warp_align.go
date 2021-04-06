package calib

import (
	"go.viam.com/robotcore/pointcloud"
	"go.viam.com/robotcore/rimage"

	"github.com/edaniels/golog"
)

type DepthColorAligner interface {
	ToAlignedImageWithDepth(*rimage.ImageWithDepth, golog.Logger) (*rimage.ImageWithDepth, error)
	ToPointCloudWithColor(*rimage.ImageWithDepth, golog.Logger) (*pointcloud.PointCloud, error)
}
