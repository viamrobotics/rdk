package slam

import (
	"math"

	"go.viam.com/robotcore/pointcloud"
)

type PointStorage interface {
	At(x, y float64) int
	Set(x, y float64, v int) error
	Unset(x, y float64)
}

type RoundingPointCloud struct {
	*pointcloud.PointCloud
}

func (cloud RoundingPointCloud) At(x, y float64) int {
	return cloud.At(math.Round(x), math.Round(y), 0)
}

func (cloud RoundingPointCloud) Unset(x, y float64) {
	cloud.Unset(math.Round(x), math.Round(y), 0)
}

func (cloud RoundingPointCloud) Set(x, y float64, v int) {
	return cloud.Set(pointcloud.NewValuePoint(math.Round(x), math.Round(y), 0, v))
}
