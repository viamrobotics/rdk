// Package pointcloud defines a point cloud and provides an implementation for one.
//
// Its implementation is dictionary based is not yet efficient. The current focus is
// to make it useful and as such the API is experimental and subject to change
// considerably.
package pointcloud

import (
	"math"

	"github.com/golang/geo/r3"
)

// MetaData is data about what's stored in the point cloud.
type MetaData struct {
	HasColor bool
	HasValue bool

	MinX, MaxX float64
	MinY, MaxY float64
	MinZ, MaxZ float64
}

// PointCloud is a general purpose container of points. It does not
// dictate whether or not the cloud is sparse or dense. The current
// basic implementation is sparse however.
type PointCloud interface {
	// Size returns the number of points in the cloud.
	Size() int

	// MetaData returns meta data
	MetaData() MetaData

	// Set places the given point in the cloud.
	Set(p r3.Vector, d Data) error

	// Unset removes a point from the cloud exists at the given position.
	// If the point does not exist, this does nothing.
	Unset(x, y, z float64)

	// At returns the point in the cloud at the given position.
	// The 2nd return is if the point exists, the first is data if any.
	At(x, y, z float64) (Data, bool)

	// Iterate iterates over all points in the cloud and calls the given
	// function for each point. If the supplied function returns false,
	// iteration will stop after the function returns.
	// numBatches lets you divide up he work. 0 means don't divide
	// myBatch is used iff numBatches > 0 and is which batch you want
	Iterate(numBatches, myBatch int, fn func(p r3.Vector, d Data) bool)
}

// NewMetaData creates a new MetaData.
func NewMetaData() MetaData {
	return MetaData{
		MinX: math.MaxFloat64,
		MinY: math.MaxFloat64,
		MinZ: math.MaxFloat64,
		MaxX: -math.MaxFloat64,
		MaxY: -math.MaxFloat64,
		MaxZ: -math.MaxFloat64,
	}
}

// Merge updates the meta data with the new data.
func (meta *MetaData) Merge(v r3.Vector, data Data) {
	if data != nil {
		if data.HasColor() {
			meta.HasColor = true
		}
		if data.HasValue() {
			meta.HasValue = true
		}
	}

	if v.X > meta.MaxX {
		meta.MaxX = v.X
	}
	if v.Y > meta.MaxY {
		meta.MaxY = v.Y
	}
	if v.Z > meta.MaxZ {
		meta.MaxZ = v.Z
	}

	if v.X < meta.MinX {
		meta.MinX = v.X
	}
	if v.Y < meta.MinY {
		meta.MinY = v.Y
	}
	if v.Z < meta.MinZ {
		meta.MinZ = v.Z
	}
}

// CloudContains is a silly helper method.
func CloudContains(cloud PointCloud, x, y, z float64) bool {
	_, got := cloud.At(x, y, z)
	return got
}
