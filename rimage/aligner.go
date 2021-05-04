package rimage

import (
	"fmt"
	"image/color"

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
		return defaultToPointCloud(i)
	}
	return i.aligner.ImageWithDepthToPointCloud(i)
}

// Projections to pointclouds are done in a naive way that don't take any camera parameters into account
func defaultToPointCloud(ii *ImageWithDepth) (pointcloud.PointCloud, error) {
	if !ii.IsAligned() {
		return nil, fmt.Errorf("input ImageWithDepth is not aligned")
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
