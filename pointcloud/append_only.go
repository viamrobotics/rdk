package pointcloud

import (
	"errors"

	"github.com/golang/geo/r3"
)

// NewAppendOnlyOnlyPointsPointCloud creates a point cloud that only can be appended to and iterated.
// It also can't have any meta data with it.
func NewAppendOnlyOnlyPointsPointCloud(allocSize int) PointCloud {
	return &appendOnlyOnlyPointsPointCloud{
		points: make([]r3.Vector, 0, allocSize),
	}
}

type appendOnlyOnlyPointsPointCloud struct {
	points []r3.Vector
}

func (pc *appendOnlyOnlyPointsPointCloud) Size() int {
	return len(pc.points)
}

func (pc *appendOnlyOnlyPointsPointCloud) MetaData() MetaData {
	panic(1)
}

func (pc *appendOnlyOnlyPointsPointCloud) Set(p r3.Vector, d Data) error {
	if d != nil {
		return errors.New("no data supported in appendOnlyOnlyPointsPointCloud")
	}
	pc.points = append(pc.points, p)
	return nil
}

func (pc *appendOnlyOnlyPointsPointCloud) At(x, y, z float64) (Data, bool) {
	panic("can't At appendOnlyOnlyPointsPointCloud")
}

func (pc *appendOnlyOnlyPointsPointCloud) Iterate(numBatches, myBatch int, fn func(p r3.Vector, d Data) bool) {
	for idx, p := range pc.points {
		if numBatches > 0 && idx%numBatches != myBatch {
			continue
		}
		if !fn(p, nil) {
			return
		}
	}
}
