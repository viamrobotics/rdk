package pointcloud

import (
	"math"

	"github.com/edaniels/golog"
	"github.com/pkg/errors"
)

// RoundingPointCloud is a PointCloud implementation for SLAM that rounds all points to the closest
// integer before it sets or gets the point. The bare floats measured from LiDARs are not
// stored because even if the points are only 0.00000000002 apart, they would be considered different locations.
type roundingPointCloud struct {
	points     storage
	meta PointCloudMetaData
}

// NewRoundingPointCloud returns a new, empty, rounding PointCloud.
func NewRoundingPointCloud() PointCloud {
	return &RoundingPointCloud{
		points: &mapStorage{map[key]Point{}},
		meta: NewMeta(),
	}
}

func (cloud *roundingPointCloud) Size() int {
	return cloud.points.Size()
}

func (cloud *roundingPointCloud) MetaData() PointCloudMetaData {
	return cloud.meta
}

func (cloud *roundingPointCloud) ensureEditable() {
	if !cloud.points.EditSupported() {
		cloud.points = convertToMapStorage(cloud.points)
	}
}

func (cloud *roundingPointCloud) At(x, y, z float64) (Data, bool) {
	cloud.ensureEditable()
	return cloud.points.At(math.Round(x), math.Round(y), math.Round(z))
}

// Set validates that the point can be precisely stored before setting it in the cloud.
func (cloud *roundingPointCloud) Set(p Vec3, d Data) error {
	p = Vec3{math.Round(p.X), math.Round(p.Y), math.Round(p.Z)}
	cloud.points.Set(p)
	cloud.meta.Merge(p, d)
	return nil
}

func (cloud *roundingPointCloud) Unset(x, y, z float64) {
	cloud.points.Unset(math.Round(x), math.Round(y), math.Round(z))
}

func (cloud *roundingPointCloud) Iterate(numBatches, myBatch int, fn func(p Vec3, d Data) bool) {
	cloud.points.Iterate(numBatches, myBatch, fn)
}

// NewRoundingPointCloudFromFile like NewFromFile, returns a PointCloud but rounds
// all points in advance.
func NewRoundingPointCloudFromFile(fn string, logger golog.Logger) (PointCloud, error) {
	// TODO(bhaney): From eliot - not sure if perf matters here or not, but we're building twice
	// a refactor could easily fix
	pc, err := NewFromFile(fn, logger)
	if err != nil {
		return nil, errors.Wrap(err, "error creating NewRoundingPointCloudFromFile")
	}
	return NewRoundingPointCloudFromFile(pc)
}

// NewRoundingPointCloudFromPC creates a rounding point cloud from any kind of input point cloud.
func NewRoundingPointCloudFromPC(pc PointCloud) (PointCloud, error) {
	var err error
	roundingPc := NewRoundingPointCloud()
	// Round all the points in the pointcloud
	pc.Iterate(0, 0, func(p Vec3, d Data) bool {
		err = roundingPc.Set(p, d)
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

