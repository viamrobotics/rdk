package slam

import (
	"fmt"
	"image/color"
	"math"

	"go.viam.com/robotcore/pointcloud"

	"github.com/edaniels/golog"
)

// SLAM stores and retrieves its visited locations as points as defined in pointcloud/point.go.
type PointStorage interface {
	Size() int
	At(x, y float64) pointcloud.Point
	Set(x, y float64, v int) error
	Unset(x, y float64)
	Iterate(fn func(p pointcloud.Point) bool)
	WriteToFile(fn string) error
}

// RoundingPointCloud is a simple PointStorage implementation that rounds all points to the closest
// integer before it sets or gets the location. The bare floats measured from devices are not
// stored because even if the points are only 0.00000000002 apart, they would be considered different
// locations.
type RoundingPointCloud struct {
	pc *pointcloud.PointCloud
}

func NewRoundingPointCloud() *RoundingPointCloud {
	return &RoundingPointCloud{pointcloud.New()}
}

func NewRoundingPointCloudFromFile(fn string, logger golog.Logger) (*RoundingPointCloud, error) {
	var err error
	pc, err := pointcloud.NewFromFile(fn, logger)
	if err != nil {
		return nil, fmt.Errorf("error creating NewRoundingPointCloudFromFile - %s", err)
	}
	// Round all the points in the pointcloud
	newPc := pointcloud.New()
	pc.Iterate(func(pt pointcloud.Point) bool {
		var ptTransformed pointcloud.Point
		pos := pt.Position()
		x, y, z := math.Round(pos.X), math.Round(pos.Y), math.Round(pos.Z)
		if pt.HasValue() {
			ptTransformed = pointcloud.NewValuePoint(x, y, z, pt.Value())
		} else if pt.HasColor() {
			ptTransformed = pointcloud.NewColoredPoint(x, y, z, pt.Color().(color.NRGBA))
		} else {
			ptTransformed = pointcloud.NewBasicPoint(x, y, z)
		}
		err = newPc.Set(ptTransformed)
		if err != nil {
			err = fmt.Errorf("error setting point (%v, %v, %v) in point cloud - %s", x, y, z, err)
			return false
		}
		return true
	})
	if err != nil {
		return nil, err
	}
	return &RoundingPointCloud{newPc}, nil
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
