package pointcloud

import (
	"container/list"
	"math"
	"sort"

	"github.com/golang/geo/r3"
	"gonum.org/v1/gonum/mat"
	"gonum.org/v1/gonum/stat"
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

	visitedMap := make(map[VoxelCoords]bool)
	visitedMap[coords] = false
	return &VoxelGrid{
		Voxels:   voxelMap,
		maxLabel: 0,
	}
}

// helpers for Voxel attributes computation

// EstimatePlaneNormalFromPoints estimates the normal vector of the plane formed by the points in the []r3.Vector
func EstimatePlaneNormalFromPoints(points []r3.Vector) r3.Vector {
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

		return r3.Vector{}
	}
	var vecs mat.Dense
	pc.VectorsTo(&vecs)
	// vectors are ordered by decreasing eigenvalues
	// the normal vector corresponds to the vector associated with the smallest eigenvalue
	// ie the last column in the vecs 3x3 matrix
	normalData := vecs.ColView(2)
	normal := r3.Vector{
		X: normalData.At(0, 0),
		Y: normalData.At(1, 0),
		Z: normalData.At(2, 0),
	}
	orientation := r3.Vector{1., 1., 1.}
	// orient normal vectors consistently
	if normal.Dot(orientation) < 0. {
		normal = normal.Mul(-1.0)
	}
	return normal.Normalize()
}

// GetVoxelCenter computes the barycenter of the points in the slice of r3.Vector
func GetVoxelCenter(points []r3.Vector) r3.Vector {
	center := r3.Vector{}
	for _, pt := range points {
		center.X = center.X + pt.X
		center.Y = center.Y + pt.Y
		center.Z = center.Z + pt.Z
	}
	center = center.Mul(1. / float64(len(points)))
	return center
}

// GetOffset computes the offset of the plane with given normal vector and a point in it
func GetOffset(center, normal r3.Vector) float64 {
	return -normal.Dot(center)
}

// DistToPlane computes the distance between a point a plane with given normal vector and offset
func DistToPlane(pt, planeNormal r3.Vector, offset float64) float64 {
	num := math.Abs(pt.Dot(planeNormal) + offset)
	denom := planeNormal.Norm()
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
		d := DistToPlane(pt, normal, offset)
		dist = dist + d*d
	}
	dist = dist / float64(len(points))
	return math.Sqrt(dist)
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

// LabelVoxels performs voxel plane labeling
// If a voxel contains points from one plane, voxel propagation is done to the neighboring voxels that are also planar
// and share the same plane equation
func (vg *VoxelGrid) LabelVoxels(sortedKeys []VoxelCoords, wTh, thetaTh, phiTh float64) {
	currentLabel := 1
	visited := make(map[VoxelCoords]bool)
	//nZeroWeight := 0
	for _, k := range sortedKeys {
		// If current voxel has a weight above threshold (plane data is relevant)
		// and has not been visited yet
		if vg.Voxels[k].Weight > wTh && !visited[k] && vg.Voxels[k].Label == 0 {
			// BFS traversal
			vg.LabelComponentBFS(vg.Voxels[k], currentLabel, wTh, thetaTh, phiTh, visited)
			vg.maxLabel = currentLabel
			currentLabel = currentLabel + 1
		}

	}
}

// LabelComponentBFS is a helper function to perform BFS per connected component
func (vg *VoxelGrid) LabelComponentBFS(vox *Voxel, label int, wTh, thetaTh, phiTh float64, visited map[VoxelCoords]bool) {
	queue := list.New()
	queue.PushBack(vox.Key)
	visited[vox.Key] = true
	for queue.Len() > 0 {
		e := queue.Front() // First element
		// interface to VoxelCoords type
		coords := e.Value.(VoxelCoords)
		// Set label of Voxel
		vg.Voxels[coords].SetLabel(label)
		// Add current key to visited set
		// Get adjacent voxels
		neighbors := vg.GetAdjacentVoxels(vg.Voxels[coords])
		for _, c := range neighbors {
			// if pair voxels satisfies smoothness and continuity constraints and
			// neighbor voxel plane data is relevant enough
			// and neighbor is not visited yet
			if vg.Voxels[coords].CanMerge(vg.Voxels[c], thetaTh, phiTh) && vg.Voxels[c].Weight > wTh && !visited[c] {
				queue.PushBack(c)
				visited[c] = true
			}
		}
		queue.Remove(e)
	}
}

// GetUnlabeledVoxels gathers in a slice all voxels whose label is 0
func (vg *VoxelGrid) GetUnlabeledVoxels() []VoxelCoords {
	unlabeled := make([]VoxelCoords, 0)
	for _, vox := range vg.Voxels {
		if vox.Label == 0 {
			unlabeled = append(unlabeled, vox.Key)
		}
	}
	return unlabeled
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

// GetPlanesFromLabels returns a slice containing all the planes in the point cloud
func (vg *VoxelGrid) GetPlanesFromLabels() ([]Plane, error) {
	planes := make([]Plane, vg.maxLabel+1)
	pointsByLabel := make(map[int][]r3.Vector)
	keysByLabel := make(map[int][]VoxelCoords)
	for _, vox := range vg.Voxels {
		currentVoxelLabel := vox.Label
		// if voxel is entirely included in a plane, add all the points
		if vox.Label > 0 {
			pointsByLabel[currentVoxelLabel] = append(pointsByLabel[currentVoxelLabel], vox.Points...)
			keysByLabel[currentVoxelLabel] = append(keysByLabel[currentVoxelLabel], vox.Key)
		} else {
			// voxel has points for either no plane or at least two planes
			// add point by point
			if len(vox.Points) == len(vox.PointLabels) {
				for ptIdx, pt := range vox.Points {
					ptLabel := vox.PointLabels[ptIdx]
					pointsByLabel[ptLabel] = append(pointsByLabel[ptLabel], pt)
				}
			}
		}
	}

	for label, pts := range pointsByLabel {
		if label > 0 {
			normalVector := EstimatePlaneNormalFromPoints(pts)
			center := GetVoxelCenter(pts)
			offset := GetOffset(center, normalVector)
			currentPlane := Plane{
				Normal:    normalVector,
				Center:    center,
				Offset:    offset,
				Points:    pts,
				VoxelKeys: keysByLabel[label],
			}
			planes = append(planes, currentPlane)
		}
	}
	return planes, nil
}

// LabelNonPlanarVoxels labels potential planar parts in Voxels that are containing more than one plane
// if a voxel contains no plane, the minimum distance of a point to one of the surrounding plane should be above
// the threshold dTh
func (vg *VoxelGrid) LabelNonPlanarVoxels(unlabeledVoxels []VoxelCoords, dTh float64) {
	for _, k := range unlabeledVoxels {
		vox := vg.Voxels[k]
		vox.PointLabels = make([]int, len(vox.Points))
		nbVoxels := vg.GetAdjacentVoxels(vox)
		for i, pt := range vox.Points {
			dMin := 100000.0
			outLabel := 0
			for _, kNb := range nbVoxels {
				voxNb := vg.Voxels[kNb]
				if voxNb.Label > 0 {

					d := DistToPlane(pt, voxNb.Normal, voxNb.Offset)
					if d < dMin {
						dMin = d
						outLabel = voxNb.Label
					}
				}
			}
			if dMin < dTh {
				vox.PointLabels[i] = outLabel
			}
		}
	}
}

// GetKeysByDecreasingOrderWeights get the voxels keys in decreasing weight order
func (vg *VoxelGrid) GetKeysByDecreasingOrderWeights() []VoxelCoords {
	// Sort voxels by weights
	s := make(VoxelSlice, 0, len(vg.Voxels))
	for _, vox := range vg.Voxels {
		s = append(s, vox)
	}
	sort.Sort(s)
	// sort in decreasing order
	ReverseVoxelSlice(s)
	// slice of keys / voxel coordinates in decreasing order
	decreasingKeys := make([]VoxelCoords, 0, len(s))
	for _, vox := range s {
		decreasingKeys = append(decreasingKeys, vox.Key)
	}
	return decreasingKeys
}

// SegmentPlanesRegionGrowing segments planes in the points in the VoxelGrid
// This segmentation only takes into account the coordinates of the points
func (vg *VoxelGrid) SegmentPlanesRegionGrowing(wTh, thetaTh, phiTh, dTh float64) {

	// Sort voxels by decreasing order of relevance weights
	decreasingKeys := vg.GetKeysByDecreasingOrderWeights()
	// Planar voxels labeling by region growing
	vg.LabelVoxels(decreasingKeys, wTh, thetaTh, phiTh)
	// For remaining voxels, labels points that are likely to belong to a plane
	unlabeledVoxels := vg.GetUnlabeledVoxels()
	vg.LabelNonPlanarVoxels(unlabeledVoxels, dTh)
}

// ConvertToPointCloudWithValue converts the voxel grid to a point cloud with values
// values are containing the labels
func (vg *VoxelGrid) ConvertToPointCloudWithValue() (PointCloud, error) {
	// fill output point cloud with labels
	pc := New()
	for _, vox := range vg.Voxels {
		for _, pt := range vox.Points {
			// create point with value
			ptValue := NewValuePoint(pt.X, pt.Y, pt.Z, vox.Label)
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
		pt := r3.Vector{p.Position().X, p.Position().Y, p.Position().Z}
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
			vox.Normal = EstimatePlaneNormalFromPoints(vox.Points)
			vox.Offset = GetOffset(vox.Center, vox.Normal)
			vox.Residual = GetResidual(vox.Points, vox.Normal, vox.Offset)
			vox.Weight = GetWeight(vox.Points, lam, vox.Residual)
		}

	}
	return voxelMap
}
