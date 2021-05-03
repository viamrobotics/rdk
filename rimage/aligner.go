package rimage

import (
	"fmt"

	"go.viam.com/robotcore/pointcloud"
)

type DepthColorAligner interface {
	AlignImageWithDepth(*ImageWithDepth) (*ImageWithDepth, error)
	ImageWithDepthToPointCloud(*ImageWithDepth) (pointcloud.PointCloud, error)
	PointCloudToImageWithDepth(pointcloud.PointCloud) (*ImageWithDepth, error)
}

func (i *ImageWithDepth) IsAligned() bool {
	return i.aligned
}

func (i *ImageWithDepth) Aligner() DepthColorAligner {
	return i.aligner
}

func (i *ImageWithDepth) SetAligner(al DepthColorAligner) {
	i.aligner = al
}

func (i *ImageWithDepth) ToPointCloud() (pointcloud.PointCloud, error) {
	if i.aligner == nil {
		return nil, fmt.Errorf("no DepthColorAligner set in ImageWithDepth for projections")
	}
	pc, err := i.aligner.ImageWithDepthToPointCloud(i)
	if err != nil {
		err = fmt.Errorf("error calling ToPointCloud() on ImageWithDepth - %s", err)
	}
	return pc, err
}
