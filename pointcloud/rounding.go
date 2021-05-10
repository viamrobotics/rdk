package pointcloud

import (
	"fmt"
	"math"

	"github.com/edaniels/golog"
)

// RoundingPointCloud is a PointCloud implementation for SLAM that rounds all points to the closest
// integer before it sets or gets the point. The bare floats measured from LiDARs are not
// stored because even if the points are only 0.00000000002 apart, they would be considered different locations.

type RoundingPointCloud struct {
	*basicPointCloud
}

func NewRoundingPointCloud() PointCloud {
	return &RoundingPointCloud{New().(*basicPointCloud)}
}

func NewRoundingPointCloudFromFile(fn string, logger golog.Logger) (PointCloud, error) {
	var err error
	roundingPc := NewRoundingPointCloud()
	pc, err := NewFromFile(fn, logger)
	if err != nil {
		return nil, fmt.Errorf("error creating NewRoundingPointCloudFromFile - %w", err)
	}
	// Round all the points in the pointcloud
	pc.Iterate(func(pt Point) bool {
		err = roundingPc.Set(pt)
		if err != nil {
			x, y, z := pt.Position().X, pt.Position().Y, pt.Position().Z
			err = fmt.Errorf("error setting point (%v, %v, %v) in point cloud - %w", x, y, z, err)
			return false
		}
		return true
	})
	return roundingPc, nil
}

func (cloud *RoundingPointCloud) Set(p Point) error {
	pos := p.Position()
	p.ChangePosition(Vec3{math.Round(pos.X), math.Round(pos.Y), math.Round(pos.Z)})
	cloud.points[key(p.Position())] = p
	if p.HasColor() {
		cloud.hasColor = true
	}
	if p.HasValue() {
		cloud.hasValue = true
	}
	v := p.Position()
	if v.X > maxPreciseFloat64 || v.X < minPreciseFloat64 {
		return newOutOfRangeErr("x", v.X)
	}
	if v.Y > maxPreciseFloat64 || v.Y < minPreciseFloat64 {
		return newOutOfRangeErr("y", v.Y)
	}
	if v.Z > maxPreciseFloat64 || v.Z < minPreciseFloat64 {
		return newOutOfRangeErr("z", v.Z)
	}
	if v.X > cloud.maxX {
		cloud.maxX = v.X
	}
	if v.Y > cloud.maxY {
		cloud.maxY = v.Y
	}
	if v.Z > cloud.maxZ {
		cloud.maxZ = v.Z
	}

	if v.X < cloud.minX {
		cloud.minX = v.X
	}
	if v.Y < cloud.minY {
		cloud.minY = v.Y
	}
	if v.Z < cloud.minZ {
		cloud.minZ = v.Z
	}
	return nil
}

func (cloud *RoundingPointCloud) At(x, y, z float64) Point {
	return cloud.points[key{math.Round(x), math.Round(y), math.Round(z)}]
}

func (cloud *RoundingPointCloud) Unset(x, y, z float64) {
	delete(cloud.points, key{math.Round(x), math.Round(y), math.Round(z)})
}
