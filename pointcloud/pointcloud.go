// Package pointcloud defines a point cloud and provides an implementation for one.
//
// Its implementation is dictionary based is not yet efficient. The current focus is
// to make it useful and as such the API is experimental and subject to change
// considerably.
package pointcloud

import (
	"math"
	"sync"

	"github.com/golang/geo/r3"
	"go.viam.com/utils"
	"gonum.org/v1/gonum/mat"
)

const numThreadsPointCloud = 8

// MetaData is data about what's stored in the point cloud.
type MetaData struct {
	HasColor bool
	HasValue bool

	MinX, MaxX             float64
	MinY, MaxY             float64
	MinZ, MaxZ             float64
	totalX, totalY, totalZ float64
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
		MinX:   math.MaxFloat64,
		MinY:   math.MaxFloat64,
		MinZ:   math.MaxFloat64,
		MaxX:   -math.MaxFloat64,
		MaxY:   -math.MaxFloat64,
		MaxZ:   -math.MaxFloat64,
		totalX: 0,
		totalY: 0,
		totalZ: 0,
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

	// Add to totals for centroid calculation.
	meta.totalX += v.X
	meta.totalY += v.Y
	meta.totalZ += v.Z
}

// CloudContains is a silly helper method.
func CloudContains(cloud PointCloud, x, y, z float64) bool {
	_, got := cloud.At(x, y, z)
	return got
}

// CloudCentroid returns the centroid of a pointcloud as a vector.
func CloudCentroid(pc PointCloud) r3.Vector {
	if pc.Size() == 0 {
		// This is done to match the centroids provided by GetObjectPointClouds.
		// Returning {NaN, NaN, NaN} is probably more correct, but this matches
		// Previous behavior.
		return r3.Vector{}
	}
	return r3.Vector{
		X: pc.MetaData().totalX / float64(pc.Size()),
		Y: pc.MetaData().totalY / float64(pc.Size()),
		Z: pc.MetaData().totalZ / float64(pc.Size()),
	}
}

type CloudMatrixCols int

const (
	// CloudMatrixColX is the x column in the cloud matrix.
	CloudMatrixColX CloudMatrixCols = 0
	// CloudMatrixColY is the y column in the cloud matrix.
	CloudMatrixColY CloudMatrixCols = 1
	// CloudMatrixColZ is the z column in the cloud matrix.
	CloudMatrixColZ CloudMatrixCols = 2
	// CloudMatrixColR is the r column in the cloud matrix.
	CloudMatrixColR CloudMatrixCols = 3
	// CloudMatrixColG is the g column in the cloud matrix.
	CloudMatrixColG CloudMatrixCols = 4
	// CloudMatrixColB is the b column in the cloud matrix.
	CloudMatrixColB CloudMatrixCols = 5
	// CloudMatrixColValue is the value column in the cloud matrix.
	CloudMatrixColV CloudMatrixCols = 6
)

func CloudMatrix(pc PointCloud) *mat.Dense {
	header := []CloudMatrixCols{CloudMatrixColX, CloudMatrixColY, CloudMatrixColZ}
	pointSize := 3 // x, y, z
	if pc.MetaData().HasColor {
		pointSize += 3 // color
		header = append(header, CloudMatrixColR, CloudMatrixColG, CloudMatrixColB)
	}
	if pc.MetaData().HasValue {
		pointSize += 1 // value
		header = append(header, CloudMatrixColV)
	}
	matrix := mat.NewDense(pc.Size(), pointSize, nil)

	var wg sync.WaitGroup
	wg.Add(numThreadsPointCloud)
	for thread := 0; thread < numThreadsPointCloud; thread++ {
		f := func(thread int) {
			defer wg.Done()
			offset := 0
			batchSize := (pc.Size() + numThreadsPointCloud - 1) / numThreadsPointCloud
			pc.Iterate(numThreadsPointCloud, thread, func(p r3.Vector, d Data) bool {
				row := thread*batchSize + offset
				offset++
				colIndex := 0
				matrix.Set(row, colIndex, p.X)
				colIndex++
				matrix.Set(row, colIndex, p.Y)
				colIndex++
				matrix.Set(row, colIndex, p.Z)
				colIndex++
				if pc.MetaData().HasColor {
					r, g, b := d.RGB255()
					matrix.Set(row, colIndex, float64(r))
					colIndex++
					matrix.Set(row, colIndex, float64(g))
					colIndex++
					matrix.Set(row, colIndex, float64(b))
					colIndex++
				}
				if pc.MetaData().HasValue {
					matrix.Set(row, colIndex, float64(d.Value()))
					colIndex++
				}
				return true
			})
		}
		threadCopy := thread
		utils.PanicCapturingGo(func() { f(threadCopy) })
	}
	return matrix
}
