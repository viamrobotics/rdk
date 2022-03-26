// Package pointcloud defines a point cloud and provides an implementation for one.
//
// Its implementation is dictionary based is not yet efficient. The current focus is
// to make it useful and as such the API is experimental and subject to change
// considerably.
package pointcloud

// PointCloudMetaData is data about what's stored in the point cloud
type PointCloudMetaData struct {
	HasColor bool
	HasValue bool

	MinX, MaxX float64
	MinY, MaxY float64
	MinZ, MaxZ float64

	inited bool // just to prevent someone creating the wrong way
}

// PointCloud is a general purpose container of points. It does not
// dictate whether or not the cloud is sparse or dense. The current
// basic implementation is sparse however.
type PointCloud interface {
	// Size returns the number of points in the cloud.
	Size() int

	// MetaData returns meta data
	MetaData() PointCloudMetaData

	// Set places the given point in the cloud.
	Set(p Vec3, d Data) error

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
	Iterate(numBatches, myBatch int, fn func(p Vec3, d Data) bool)
}

func NewMeta() (PointCloudMetaData) {
	return PointCloudMetaData{
		minX:   math.MaxFloat64,
		minY:   math.MaxFloat64,
		minZ:   math.MaxFloat64,
		maxX:   -math.MaxFloat64,
		maxY:   -math.MaxFloat64,
		maxZ:   -math.MaxFloat64,
	}
}

func (meta *PointCloudMetaData) Merge(p Vec3, data Data) {
	if data.HasColor() {
		meta.hasColor = true
	}
	if data.HasValue() {
		meta.hasValue = true
	}

	v := p.Position()
	
	if v.X > meta.maxX {
		meta.maxX = v.X
	}
	if v.Y > meta.maxY {
		meta.maxY = v.Y
	}
	if v.Z > meta.maxZ {
		meta.maxZ = v.Z
	}

	if v.X < meta.minX {
		meta.minX = v.X
	}
	if v.Y < meta.minY {
		meta.minY = v.Y
	}
	if v.Z < meta.minZ {
		meta.minZ = v.Z
	}

}
