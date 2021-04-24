package slam

import (
	"fmt"
	"math"

	"go.viam.com/robotcore/pointcloud"

	"github.com/edaniels/golog"
)

type PointStorage interface {
	Size() int
	At(x, y float64) pointcloud.Point
	Set(x, y float64, v int) error
	Unset(x, y float64)
	Iterate(fn func(p pointcloud.Point) bool)
	WriteToFile(fn string) error
}

type RoundingPointCloud struct {
	pc *pointcloud.PointCloud
}

func NewRoundingPointCloud() *RoundingPointCloud {
	return &RoundingPointCloud{pointcloud.New()}
}

func NewRoundingPointCloudFromFile(fn string, logger golog.Logger) (*RoundingPointCloud, error) {
	pc, err := pointcloud.NewFromFile(fn, logger)
	if err != nil {
		return nil, fmt.Errorf("error creating NewRoundingPointCloudFromFile - %s", err)
	}
	return &RoundingPointCloud{pc}, nil
}

func (cloud *RoundingPointCloud) Size() int {
	return cloud.pc.Size()
}

func (cloud *RoundingPointCloud) At(x, y float64) pointcloud.Point {
	return cloud.pc.At(math.Round(x), math.Round(y), 0)
}

func (cloud *RoundingPointCloud) Set(x, y float64, v int) error {
	return cloud.pc.Set(pointcloud.NewValuePoint(math.Round(x), math.Round(y), 0, v))
}

func (cloud *RoundingPointCloud) Unset(x, y float64) {
	cloud.pc.Unset(math.Round(x), math.Round(y), 0)
}

func (cloud *RoundingPointCloud) WriteToFile(fn string) error {
	return cloud.pc.WriteToFile(fn)
}

func (cloud *RoundingPointCloud) Iterate(fn func(p pointcloud.Point) bool) {
	cloud.pc.Iterate(fn)
}
