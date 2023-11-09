package pointcloud

import (
	"math"

	"github.com/golang/geo/r3"
	"github.com/pkg/errors"

	"go.viam.com/rdk/logging"
)

// RoundingPointCloud is a PointCloud implementation for SLAM that rounds all points to the closest
// integer before it sets or gets the point. The bare floats measured from LiDARs are not
// stored because even if the points are only 0.00000000002 apart, they would be considered different locations.
type roundingPointCloud struct {
	points storage
	meta   MetaData
}

// NewRoundingPointCloud returns a new, empty, rounding PointCloud.
func NewRoundingPointCloud() PointCloud {
	return &roundingPointCloud{
		points: &matrixStorage{points: []PointAndData{}, indexMap: map[r3.Vector]uint{}},
		meta:   NewMetaData(),
	}
}

func (cloud *roundingPointCloud) Size() int {
	return cloud.points.Size()
}

func (cloud *roundingPointCloud) MetaData() MetaData {
	return cloud.meta
}

func (cloud *roundingPointCloud) At(x, y, z float64) (Data, bool) {
	return cloud.points.At(math.Round(x), math.Round(y), math.Round(z))
}

// Set validates that the point can be precisely stored before setting it in the cloud.
func (cloud *roundingPointCloud) Set(p r3.Vector, d Data) error {
	p = r3.Vector{math.Round(p.X), math.Round(p.Y), math.Round(p.Z)}
	_, pointExists := cloud.At(p.X, p.Y, p.Z)
	if err := cloud.points.Set(p, d); err != nil {
		return err
	}
	if !pointExists {
		cloud.meta.Merge(p, d)
	}
	return nil
}

func (cloud *roundingPointCloud) Iterate(numBatches, myBatch int, fn func(p r3.Vector, d Data) bool) {
	cloud.points.Iterate(numBatches, myBatch, fn)
}

// NewRoundingPointCloudFromFile like NewFromFile, returns a PointCloud but rounds
// all points in advance.
func NewRoundingPointCloudFromFile(fn string, logger logging.Logger) (PointCloud, error) {
	// TODO(bhaney): From eliot - not sure if perf matters here or not, but we're building twice
	// a refactor could easily fix
	pc, err := NewFromFile(fn, logger)
	if err != nil {
		return nil, errors.Wrap(err, "error creating NewRoundingPointCloudFromFile")
	}
	return NewRoundingPointCloudFromPC(pc)
}

// NewRoundingPointCloudFromPC creates a rounding point cloud from any kind of input point cloud.
func NewRoundingPointCloudFromPC(pc PointCloud) (PointCloud, error) {
	var err error
	roundingPc := NewRoundingPointCloud()
	// Round all the points in the pointcloud
	pc.Iterate(0, 0, func(p r3.Vector, d Data) bool {
		err = roundingPc.Set(p, d)
		if err != nil {
			x, y, z := p.X, p.Y, p.Z
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
