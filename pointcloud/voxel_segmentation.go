package pointcloud

import (
	"container/list"
	"fmt"
	"math"

	"github.com/golang/geo/r3"
	"gonum.org/v1/gonum/mat"
	"gonum.org/v1/gonum/stat"
)

// VoxelCoords stores Voxel coordinates in VoxelGrid axes
type VoxelCoords struct {
	I, J, K int64
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
func (c VoxelCoords) IsEqual(c2 VoxelCoords) bool {
	return c.I == c2.I && c.J == c2.J && c.K == c2.K
}

type VoxelSlice []*Voxel
type PointSlice []r3.Vector
type VoxelGrid struct {
	Voxels  map[VoxelCoords]*Voxel
	visited map[VoxelCoords]bool
}

func NewVoxelGrid() *VoxelGrid {
	voxelMap := make(map[VoxelCoords]*Voxel)
	coords := VoxelCoords{
		I: 0,
		J: 0,
		K: 0,
	}
	voxelMap[coords] = NewVoxel(coords)

	visitedMap := make(map[VoxelCoords]bool)
	visitedMap[coords] = false
	return &VoxelGrid{
		Voxels:  voxelMap,
		visited: visitedMap,
	}
}

// EstimateNormal estimates the normal vector of the plane formed by the points in the PointSlice
func EstimateNormal(points PointSlice) r3.Vector {
	// Put points in mat
	nPoints := len(points)
	mPt := mat.NewDense(nPoints, 3, nil)
	for i, v := range points {
		mPt.Set(i, 0, v.X)
		mPt.Set(i, 1, v.Y)
		mPt.Set(i, 2, v.Z)
	}
	// Compute PCA
	var pc stat.PC
	ok := pc.PrincipalComponents(mPt, nil)
	if !ok {
		fmt.Println("error processing PCA on points")
	}
	var vecs mat.Dense
	pc.VectorsTo(&vecs)

	normalData := vecs.ColView(2)
	normal := r3.Vector{normalData.At(0, 0), normalData.At(1, 0), normalData.At(2, 0)}
	return normal.Normalize()
}

// EstimateCenter computes the barycenter of the points in the PointSlice
func EstimateCenter(points PointSlice) r3.Vector {
	center := r3.Vector{}
	for _, pt := range points {
		center.X = center.X + pt.X
		center.Y = center.Y + pt.Y
		center.Z = center.Z + pt.Z
	}
	center.Mul(1. / float64(len(points)))
	return center
}

// GetOffset computes the offset of the plane with given normal vector and a point in it
func GetOffset(center, normal r3.Vector) float64 {
	return -normal.Dot(center)
}

// DistToPlane computes the distance between a point a plane with given normal vector and offset
func DistToPlane(pt, normal r3.Vector, offset float64) float64 {
	num := math.Abs(pt.Dot(normal) + offset)
	denom := normal.Norm()
	d := 0.
	if denom > 0.0001 {
		d = num / denom
	}
	return d
}

// GetResidual computes the mean fitting error of points to a given plane
func GetResidual(points []r3.Vector, normal r3.Vector, offset float64) float64 {
	dist := 0.
	for _, pt := range points {
		dist = dist + DistToPlane(pt, normal, offset)*DistToPlane(pt, normal, offset)
	}
	dist = dist / float64(len(points))
	return math.Sqrt(dist)
}

// GetPtMin gets the minimum coordinates of a slice of points on each axis
func GetPtMin(points []r3.Vector) r3.Vector {
	ptMin := r3.Vector{10000, 10000, 10000}
	for _, pt := range points {
		if pt.X < ptMin.X {
			ptMin.X = pt.X
		}
		if pt.Y < ptMin.Y {
			ptMin.Y = pt.Y
		}
		if pt.Z < ptMin.Z {
			ptMin.Z = pt.Z
		}
	}
	return ptMin
}

// GetVoxelCoordinates computes voxel coordinates in VoxelGrid Axes
func GetVoxelCoordinates(pt, ptMin r3.Vector, voxelSize float64) VoxelCoords {
	ptVoxel := pt.Sub(ptMin)
	coords := VoxelCoords{}
	coords.I = int64(math.Floor(ptVoxel.X / voxelSize))
	coords.J = int64(math.Floor(ptVoxel.Y / voxelSize))
	coords.K = int64(math.Floor(ptVoxel.Z / voxelSize))
	return coords
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

// GetWeight computes weights for Region Growing segmentation
func GetWeight(points []r3.Vector, lam, residual float64) float64 {
	nPoints := len(points)
	dR := 1. / float64(nPoints)
	w := math.Exp(-dR*dR/(2*lam*lam)) * math.Exp(-residual*residual/(2*lam*lam))
	return w
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

// IsSmooth returns true if two voxels respect the smoothness constraint, false otherwise
// angleTh is expressed in degrees
func (v1 *Voxel) IsSmooth(v2 *Voxel, angleTh float64) bool {
	angle := math.Abs(v1.Normal.Dot(v2.Normal))
	angle = math.Abs(math.Acos(angle))
	//angle := v1.Normal.Dot(v2.Normal)
	//angle = math.Acos(angle)
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

// GetVoxelFromKey returns a pointer to a voxel from a VoxelCoords key
func (vg *VoxelGrid) GetVoxelFromKey(coords VoxelCoords) *Voxel {
	return vg.Voxels[coords]
}

// GetAdjacentVoxels gets adjacent voxels in point cloud in 26-connectivity
func (vg VoxelGrid) GetAdjacentVoxels(v *Voxel) []VoxelCoords {
	i, j, k := v.Key.I, v.Key.J, v.Key.K
	is := []int64{i - 1, i, i + 1}
	js := []int64{j - 1, j, j + 1}
	ks := []int64{k - 1, k, k + 1}
	neighborKeys := make([]VoxelCoords, 0)
	for i_ := range is {
		for j_ := range js {
			for k_ := range ks {
				vox := VoxelCoords{int64(i_), int64(j_), int64(k_)}
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

// LabelVoxels performs voxel plane labeling
func (vg *VoxelGrid) LabelVoxels(sortedKeys []VoxelCoords, wTh, thetaTh, phiTh float64) {
	currentLabel := 1
	for _, key := range sortedKeys {
		// If current voxel has a weight above threshold (plane data is relevant)
		// and has not been visited yet (label == 0)
		if vg.Voxels[key].Weight > wTh && !vg.visited[key] {
			//if vg[key].Label == 0 {
			// BFS
			//vg.LabelComponentBFS(vg[key], currentLabel, wTh, thetaTh, phiTh)
			queue := list.New()
			queue.PushBack(vg.Voxels[key].Key)
			for queue.Len() > 0 {
				e := queue.Front() // First element
				// interface to VoxelCoords type
				coords := VoxelCoords(e.Value.(VoxelCoords))
				// Set label of Voxel
				vg.Voxels[coords].Label = currentLabel
				// Add current key to visited set
				vg.visited[coords] = true
				// Get adjacent voxels
				neighbors := vg.GetAdjacentVoxels(vg.Voxels[coords])
				fmt.Println("Neighbors : ", len(neighbors))
				for _, c := range neighbors {
					// if pair voxels satisfies smoothness and continuity constraints and
					// neighbor voxel plane data is relevant enough
					// and neighbor is not visited yet
					//if vg[coords].CanMerge(vg[c], thetaTh, phiTh) && vg[c].Weight > wTh  && vg[c].Label == 0{
					//if vg[coords].CanMerge(vg[c], thetaTh, phiTh){
					if vg.Voxels[coords].CanMerge(vg.Voxels[c], thetaTh, phiTh) && !vg.visited[c] {
						fmt.Println("Merging!")
						queue.PushBack(c)
					}
				}
				queue.Remove(e)
			}
			currentLabel = currentLabel + 1
		}
	}
}

// LabelComponentBFS is a helper function to perform BFS per connected component
func (vg VoxelGrid) LabelComponentBFS(vox *Voxel, label int, wTh, thetaTh, phiTh float64) {
	queue := list.New()
	queue.PushBack(vox.Key)
	for queue.Len() > 0 {
		e := queue.Front() // First element
		// interface to VoxelCoords type
		coords := VoxelCoords(e.Value.(VoxelCoords))
		// Set label of Voxel
		vg.Voxels[coords].Label = label
		// Add current key to visited set
		vg.visited[coords] = true
		// Get adjacent voxels
		neighbors := vg.GetAdjacentVoxels(vg.Voxels[coords])
		fmt.Println("Neighbors : ", len(neighbors))
		for _, c := range neighbors {
			// if pair voxels satisfies smoothness and continuity constraints and
			// neighbor voxel plane data is relevant enough
			// and neighbor is not visited yet
			//if vg[coords].CanMerge(vg[c], thetaTh, phiTh) && vg[c].Weight > wTh  && vg[c].Label == 0{
			//if vg[coords].CanMerge(vg[c], thetaTh, phiTh){
			if vg.Voxels[coords].CanMerge(vg.Voxels[c], thetaTh, phiTh) && !vg.visited[c] {
				fmt.Println("Merging!")
				queue.PushBack(c)
			}
		}
		queue.Remove(e)
	}
}

func (vg *VoxelGrid) GetUnlabeledVoxels() []*Voxel {
	unlabeled := make([]*Voxel, 0)
	for _, vox := range vg.Voxels {
		if vox.Label == 0 {
			unlabeled = append(unlabeled, vox)
		}
	}
	return unlabeled
}
