package rimage

import (
	"fmt"
	"io"

	"go.viam.com/robotcore/pointcloud"
)

// Naive implementation that does not take into account camera parameters
// If you have the Camera matrices, use calib.DepthMapToPointCloud
func (dm *DepthMap) ToPointCloud() (*pointcloud.PointCloud, error) {
	pc := pointcloud.New()

	height := dm.Height()
	width := dm.Width()
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			z := dm.GetDepth(x, y)
			if z == 0 {
				continue
			}
			err := pc.Set(pointcloud.NewBasicPoint(float64(x), float64(y), float64(z)))
			if err != nil {
				return nil, err
			}
		}
	}
	return pc, nil
}
