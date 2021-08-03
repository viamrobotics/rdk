package pointcloud

import (
	"math"

	"github.com/golang/geo/r3"
)

/* In this file are functions to create a Voxel, a Voxel Grid from a point cloud
A Voxel is a a voxel represents a value on a regular grid in
three-dimensional space. As with pixels in a 2D bitmap, voxels themselves do
not typically have their position (i.e. coordinates) explicitly encoded with
their values.
More information and comparisons with pixels here:
- https://en.wikipedia.org/wiki/Voxel
- https://medium.com/retronator-magazine/pixels-and-voxels-the-long-answer-5889ecc18190
*/

// VoxelCoords stores Voxel coordinates in VoxelGrid axes
type VoxelCoords struct {
	I, J, K int64
}

// Plane structure to store normal vector and offset of plane equation
// Additionally, it can store points composing the plane and the keys of the voxels entirely included in the plane
type Plane struct {
	Normal    r3.Vector
	Center    r3.Vector
	Offset    float64
	Points    []r3.Vector
	VoxelKeys []VoxelCoords
}

// GetEquation return the coefficients of the plane equation as a 4-slice of floats
func (p *Plane) GetEquation() []float64 {
	equation := make([]float64, 4)
	equation[0] = p.Normal.X
	equation[1] = p.Normal.Y
	equation[2] = p.Normal.Z
	equation[3] = p.Offset
	return equation
}

// IsEqual tests if two VoxelCoords are the same
func (c VoxelCoords) IsEqual(c2 VoxelCoords) bool {
	return c.I == c2.I && c.J == c2.J && c.K == c2.K
}

// Voxel is the structure to store data relevant to Voxel operations in point clouds
type Voxel struct {
	Key             VoxelCoords
	Label           int
	Points          []r3.Vector
	Center          r3.Vector
	Normal          r3.Vector
	Offset          float64
	Residual        float64
	Weight          float64
	SortedWeightIdx int
	PointLabels     []int
}

// NewVoxel creates a pointer to a Voxel struct
func NewVoxel(coords VoxelCoords) *Voxel {
	return &Voxel{
		Key:             coords,
		Label:           0,
		Points:          make([]r3.Vector, 0),
		Center:          r3.Vector{},
		Normal:          r3.Vector{},
		Offset:          0,
		Residual:        100000,
		Weight:          0,
		SortedWeightIdx: 0,
		PointLabels:     nil,
	}
}

// NewVoxelFromPoint creates a new voxel from a point
func NewVoxelFromPoint(pt, ptMin r3.Vector, voxelSize float64) *Voxel {
	coords := GetVoxelCoordinates(pt, ptMin, voxelSize)
	return &Voxel{
		Key:             coords,
		Label:           0,
		Points:          []r3.Vector{pt},
		Center:          r3.Vector{},
		Normal:          r3.Vector{},
		Offset:          0,
		Residual:        0,
		Weight:          0,
		SortedWeightIdx: 0,
		PointLabels:     nil,
	}
}

// SetLabel sets a voxel
func (v1 *Voxel) SetLabel(label int) {
	v1.Label = label
}

// IsSmooth returns true if two voxels respect the smoothness constraint, false otherwise
// angleTh is expressed in degrees
func (v1 *Voxel) IsSmooth(v2 *Voxel, angleTh float64) bool {
	angle := math.Abs(v1.Normal.Dot(v2.Normal))
	angle = math.Abs(math.Acos(angle))
	angle = angle * 180 / math.Pi

	return angle < angleTh
}

// IsContinuous returns true if two voxels respect the continuity constraint, false otherwise
// cosTh is in [0,1]
func (v1 *Voxel) IsContinuous(v2 *Voxel, cosTh float64) bool {
	v := v2.Center.Sub(v1.Center).Normalize()
	phi := math.Abs(v.Dot(v1.Normal))
	return phi < cosTh
}

// CanMerge returns true if two voxels can be added to the same connected component
func (v1 *Voxel) CanMerge(v2 *Voxel, angleTh, cosTh float64) bool {
	return v1.IsSmooth(v2, angleTh) && v1.IsContinuous(v2, cosTh)
}

// ComputeCenter computer barycenter of points in voxel
func (v1 *Voxel) ComputeCenter() {
	center := r3.Vector{}
	for _, pt := range v1.Points {
		center.Add(pt)
	}
	center = center.Mul(1. / float64(len(v1.Points)))
	v1.Center.X = center.X
	v1.Center.Y = center.Y
	v1.Center.Z = center.Z
}

// GetPlane returns the plane struct with the voxel data
func (v1 *Voxel) GetPlane() Plane {
	// create key slice for plane struct
	keys := make([]VoxelCoords, len(v1.Points))
	for i := range keys {
		keys[i] = v1.Key
	}
	return Plane{v1.Normal, v1.Center, v1.Offset, v1.Points, keys}
}

// VoxelSlice is a slice that contains Voxels
type VoxelSlice []*Voxel

// VoxelGrid contains the sparse grid of Voxels of a point cloud
type VoxelGrid struct {
	Voxels   map[VoxelCoords]*Voxel
	maxLabel int
}

// NewVoxelGrid returns a pointer to a VoxelGrid with a (0,0,0) Voxel
func NewVoxelGrid() *VoxelGrid {
	voxelMap := make(map[VoxelCoords]*Voxel)
	coords := VoxelCoords{
		I: 0,
		J: 0,
		K: 0,
	}
	voxelMap[coords] = NewVoxel(coords)

	return &VoxelGrid{
		Voxels:   voxelMap,
		maxLabel: 0,
	}
}

// Sort interface for voxels

// Swap for VoxelSlice sorting interface
func (d VoxelSlice) Swap(i, j int) {
	d[i], d[j] = d[j], d[i]
}

// Len for VoxelSlice sorting interface
func (d VoxelSlice) Len() int {
	return len(d)
}

// Less for VoxelSlice sorting interface
func (d VoxelSlice) Less(i, j int) bool {
	return d[i].Weight < d[j].Weight
}

// ReverseVoxelSlice reverses a slice of voxels
func ReverseVoxelSlice(s VoxelSlice) {
	for i, j := 0, len(s)-1; i < j; i, j = i+1, j-1 {
		s[i], s[j] = s[j], s[i]
	}
}

// GetVoxelFromKey returns a pointer to a voxel from a VoxelCoords key
func (vg *VoxelGrid) GetVoxelFromKey(coords VoxelCoords) *Voxel {
	return vg.Voxels[coords]
}

// GetAdjacentVoxels gets adjacent voxels in point cloud in 26-connectivity
func (vg VoxelGrid) GetAdjacentVoxels(v *Voxel) []VoxelCoords {
	I, J, K := v.Key.I, v.Key.J, v.Key.K
	is := []int64{I - 1, I, I + 1}
	js := []int64{J - 1, J, J + 1}
	ks := []int64{K - 1, K, K + 1}
	neighborKeys := make([]VoxelCoords, 0)
	for _, i := range is {
		for _, j := range js {
			for _, k := range ks {
				vox := VoxelCoords{i, j, k}
				_, ok := vg.Voxels[vox]
				// if neighboring voxel is in VoxelGrid and is not current voxel
				if ok && !v.Key.IsEqual(vox) {
					neighborKeys = append(neighborKeys, vox)
				}
			}
		}
	}
	return neighborKeys
}

// ConvertToPointCloudWithValue converts the voxel grid to a point cloud with values
// values are containing the labels
func (vg *VoxelGrid) ConvertToPointCloudWithValue() (PointCloud, error) {
	// fill output point cloud with labels
	pc := New()
	for _, vox := range vg.Voxels {
		for i, pt := range vox.Points {
			label := 0
			if vox.PointLabels == nil {
				// create point with value
				label = vox.Label
			} else {
				label = vox.PointLabels[i]
			}
			ptValue := NewValuePoint(pt.X, pt.Y, pt.Z, label)
			// add it to the point cloud
			err := pc.Set(ptValue)
			if err != nil {
				return nil, err
			}
		}
	}
	return pc, nil
}

// NewVoxelGridFromPointCloud creates and fills a VoxelGrid from a point cloud
func NewVoxelGridFromPointCloud(pc PointCloud, voxelSize, lam float64) *VoxelGrid {
	voxelMap := NewVoxelGrid()
	ptMin := r3.Vector{
		X: pc.MinX(),
		Y: pc.MinY(),
		Z: pc.MinZ(),
	}

	defaultResidual := 1.0

	pc.Iterate(func(p Point) bool {
		pt := r3.Vector(p.Position())
		coords := GetVoxelCoordinates(pt, ptMin, voxelSize)
		vox, ok := voxelMap.Voxels[coords]
		// if voxel key does not exist yet, create voxel at this key with current point, voxel coordinates and maximum
		// possible residual for planes
		if !ok {
			voxelMap.Voxels[coords] = &Voxel{
				Key:             coords,
				Label:           0,
				Points:          []r3.Vector{pt},
				Center:          r3.Vector{},
				Normal:          r3.Vector{},
				Offset:          0,
				Residual:        defaultResidual,
				Weight:          0,
				SortedWeightIdx: 0,
				PointLabels:     nil,
			}
		} else {
			// if voxel coordinates is in the keys of voxelMap, add point to slice
			vox.Points = append(vox.Points, pt)
		}
		return true
	})

	// All points are now assigned to a voxel in the voxel grid
	// Compute voxel attributes
	for k, vox := range voxelMap.Voxels {
		// Voxel must have enough point to make relevant computations
		vox.Key = k
		center := GetVoxelCenter(vox.Points)
		vox.Center.X = center.X
		vox.Center.Y = center.Y
		vox.Center.Z = center.Z

		// below 5 points, normal and center estimation are not relevant
		if len(vox.Points) > 5 {
			vox.Normal = estimatePlaneNormalFromPoints(vox.Points)
			vox.Offset = GetOffset(vox.Center, vox.Normal)
			vox.Residual = GetResidual(vox.Points, vox.GetPlane())
			vox.Weight = GetWeight(vox.Points, lam, vox.Residual)
		}

	}
	return voxelMap
}
