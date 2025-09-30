// Package pointcloud defines a point cloud and provides an implementation for one.
//
// Its implementation is dictionary based is not yet efficient. The current focus is
// to make it useful and as such the API is experimental and subject to change
// considerably.
package pointcloud

import (
	"bytes"
	"fmt"
	"math"
	"sync"

	"github.com/golang/geo/r3"
	commonpb "go.viam.com/api/common/v1"
	"go.viam.com/utils"
	"gonum.org/v1/gonum/mat"

	"go.viam.com/rdk/spatialmath"
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

// PointAndData is a tiny struct to facilitate returning nearest neighbors in a neat way.
type PointAndData struct {
	P r3.Vector
	D Data
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

	// FinalizeAfterReading is called after a pc is done being read
	// often this is a noop
	FinalizeAfterReading() (PointCloud, error)

	CreateNewRecentered(offset spatialmath.Pose) PointCloud
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

// TotalX returns the totalX stored in metadata.
func (meta *MetaData) TotalX() float64 {
	return meta.totalX
}

// TotalY returns the totalY stored in metadata.
func (meta *MetaData) TotalY() float64 {
	return meta.totalY
}

// TotalZ returns the totalZ stored in metadata.
func (meta *MetaData) TotalZ() float64 {
	return meta.totalZ
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

// Center returns the center of the points.
func (meta *MetaData) Center() r3.Vector {
	return r3.Vector{
		X: (meta.MaxX + meta.MinX) / 2,
		Y: (meta.MaxY + meta.MinY) / 2,
		Z: (meta.MaxZ + meta.MinZ) / 2,
	}
}

// MaxSideLength function for calculating the max side length of a pointcloud based on its metadata.
func (meta *MetaData) MaxSideLength() float64 {
	return math.Max((meta.MaxX - meta.MinX), math.Max((meta.MaxY-meta.MinY), (meta.MaxZ-meta.MinZ)))
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

// CloudToPoints converts a point cloud to a list of points (a list of vectors).
func CloudToPoints(pc PointCloud) []r3.Vector {
	points := make([]r3.Vector, 0, pc.Size())
	pc.Iterate(0, 0, func(point r3.Vector, _ Data) bool {
		points = append(points, point)
		return true // Keep going through all points
	})
	return points
}

// CloudMatrixCol is a type that represents the columns of a CloudMatrix.
type CloudMatrixCol int

const (
	// CloudMatrixColX is the x column in the cloud matrix.
	CloudMatrixColX CloudMatrixCol = 0
	// CloudMatrixColY is the y column in the cloud matrix.
	CloudMatrixColY CloudMatrixCol = 1
	// CloudMatrixColZ is the z column in the cloud matrix.
	CloudMatrixColZ CloudMatrixCol = 2
	// CloudMatrixColR is the r column in the cloud matrix.
	CloudMatrixColR CloudMatrixCol = 3
	// CloudMatrixColG is the g column in the cloud matrix.
	CloudMatrixColG CloudMatrixCol = 4
	// CloudMatrixColB is the b column in the cloud matrix.
	CloudMatrixColB CloudMatrixCol = 5
	// CloudMatrixColV is the value column in the cloud matrix.
	CloudMatrixColV CloudMatrixCol = 6
)

// CloudMatrix Returns a Matrix representation of a Cloud along with a Header list.
// The Header list is a list of CloudMatrixCols that correspond to the columns in the matrix.
// CloudMatrix is not guaranteed to return points in the same order as the cloud.
func CloudMatrix(pc PointCloud) (*mat.Dense, []CloudMatrixCol) {
	if pc.Size() == 0 {
		return nil, nil
	}
	header := []CloudMatrixCol{CloudMatrixColX, CloudMatrixColY, CloudMatrixColZ}
	pointSize := 3 // x, y, z
	if pc.MetaData().HasColor {
		pointSize += 3 // color
		header = append(header, CloudMatrixColR, CloudMatrixColG, CloudMatrixColB)
	}
	if pc.MetaData().HasValue {
		pointSize++ // value
		header = append(header, CloudMatrixColV)
	}

	var wg sync.WaitGroup
	wg.Add(numThreadsPointCloud)
	matChan := make(chan []float64, numThreadsPointCloud)
	for thread := 0; thread < numThreadsPointCloud; thread++ {
		f := func(thread int) {
			defer wg.Done()
			batchSize := (pc.Size() + numThreadsPointCloud - 1) / numThreadsPointCloud
			buf := make([]float64, 0, pointSize*batchSize)
			pc.Iterate(numThreadsPointCloud, thread, func(p r3.Vector, d Data) bool {
				buf = append(buf, p.X, p.Y, p.Z)
				if pc.MetaData().HasColor {
					r, g, b := d.RGB255()
					buf = append(buf, float64(r), float64(g), float64(b))
				}
				if pc.MetaData().HasValue {
					buf = append(buf, float64(d.Value()))
				}
				return true
			})
			matChan <- buf
		}
		threadCopy := thread
		utils.PanicCapturingGo(func() { f(threadCopy) })
	}
	wg.Wait()
	matData := make([]float64, 0, pc.Size()*pointSize)
	for i := 0; i < numThreadsPointCloud; i++ {
		matData = append(matData, <-matChan...)
	}
	return mat.NewDense(pc.Size(), pointSize, matData), header
}

// NewPointCloudFromProto creates a new point cloud geometry from a protobuf point cloud.
func NewPointCloudFromProto(pointCloud *commonpb.PointCloud, label string) (*BasicOctree, error) {
	reader := bytes.NewReader(pointCloud.PointCloud)
	pc, err := ReadPCD(reader, BasicOctreeType)
	if err != nil {
		return nil, err
	}
	octree, ok := pc.(*BasicOctree)
	if !ok {
		return nil, fmt.Errorf("why did ReadPCD not return a BasicOctree but a %T", pc)
	}
	octree.confidenceThreshold = 0
	octree.SetLabel(label)
	return octree, nil
}
