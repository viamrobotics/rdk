package pointcloud

import (
	"math"

	"github.com/golang/geo/r3"

	"go.viam.com/rdk/spatialmath"
)

func init() {
	Register(TypeConfig{
		StructureType: "rounding",
		NewWithParams: func(size int) PointCloud { return newRoundingPointCloud() },
	})
}

// RoundingPointCloud is a PointCloud implementation for SLAM that rounds all points to the closest
// integer before it sets or gets the point. The bare floats measured from LiDARs are not
// stored because even if the points are only 0.00000000002 apart, they would be considered different locations.
type roundingPointCloud struct {
	points storage
	meta   MetaData
}

func newRoundingPointCloud() PointCloud {
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

func (cloud *roundingPointCloud) FinalizeAfterReading() (PointCloud, error) {
	return cloud, nil
}

func (cloud *roundingPointCloud) CreateNewRecentered(offset spatialmath.Pose) PointCloud {
	return newRoundingPointCloud()
}
