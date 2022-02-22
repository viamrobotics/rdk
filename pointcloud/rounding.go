package pointcloud

import (
	"math"

	"github.com/edaniels/golog"
	"github.com/pkg/errors"
)

// RoundingPointCloud is a PointCloud implementation for SLAM that rounds all points to the closest
// integer before it sets or gets the point. The bare floats measured from LiDARs are not
// stored because even if the points are only 0.00000000002 apart, they would be considered different locations.
type RoundingPointCloud struct {
	*basicPointCloud
}

// NewRoundingPointCloud returns a new, empty, rounding PointCloud.
func NewRoundingPointCloud() PointCloud {
	return &RoundingPointCloud{New().(*basicPointCloud)}
}

// NewRoundingPointCloudFromFile like NewFromFile, returns a PointCloud but rounds
// all points in advance.
func NewRoundingPointCloudFromFile(fn string, logger golog.Logger) (PointCloud, error) {
	var err error
	roundingPc := NewRoundingPointCloud()
	pc, err := NewFromFile(fn, logger)
	if err != nil {
		return nil, errors.Wrap(err, "error creating NewRoundingPointCloudFromFile")
	}
	// Round all the points in the pointcloud
	pc.Iterate(func(pt Point) bool {
		err = roundingPc.Set(pt)
		if err != nil {
			x, y, z := pt.Position().X, pt.Position().Y, pt.Position().Z
			err = errors.Wrapf(err, "error setting point (%v, %v, %v) in point cloud", x, y, z)
			return false
		}
		return true
	})
	if err != nil {
		return nil, err
	}
	return roundingPc, nil
}

// NewRoundingPointCloudFromPC creates a rounding point cloud from any kind of input point cloud.
func NewRoundingPointCloudFromPC(pc PointCloud) (PointCloud, error) {
	var err error
	roundingPc := NewRoundingPointCloud()
	// Round all the points in the pointcloud
	pc.Iterate(func(pt Point) bool {
		err = roundingPc.Set(pt)
		if err != nil {
			x, y, z := pt.Position().X, pt.Position().Y, pt.Position().Z
			err = errors.Wrapf(err, "error setting point (%v, %v, %v) in point cloud", x, y, z)
			return false
		}
		return true
	})
	if err != nil {
		return nil, err
	}
	return roundingPc, nil
}

// Set sets a point on the cloud.
func (cloud *RoundingPointCloud) Set(p Point) error {
	pos := p.Position()
	rp := p.Clone(Vec3{math.Round(pos.X), math.Round(pos.Y), math.Round(pos.Z)})
	cloud.points[key(rp.Position())] = rp
	if rp.HasColor() {
		cloud.hasColor = true
	}
	if rp.HasValue() {
		cloud.hasValue = true
	}
	v := rp.Position()
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

// At returns a point in the cloud, if exist; returns nil otherwise.
func (cloud *RoundingPointCloud) At(x, y, z float64) Point {
	return cloud.points[key{math.Round(x), math.Round(y), math.Round(z)}]
}

// Unset removes a point from the cloud; does nothing if it does not exist.
func (cloud *RoundingPointCloud) Unset(x, y, z float64) {
	delete(cloud.points, key{math.Round(x), math.Round(y), math.Round(z)})
}
