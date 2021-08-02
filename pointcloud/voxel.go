package pointcloud

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"math"

	"github.com/golang/geo/r3"
	"go-hep.org/x/hep/hbook"
	"go-hep.org/x/hep/hplot"
	vecg "gonum.org/v1/plot/vg"
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

// voxelPlane structure to store normal vector and offset of plane equation
// Additionally, it can store points composing the plane and the keys of the voxels entirely included in the plane
type voxelPlane struct {
	normal    r3.Vector
	center    r3.Vector
	offset    float64
	points    []Point
	voxelKeys []VoxelCoords
}

func NewPlaneFromVoxel(normal, center r3.Vector, offset float64, points []Point, voxelKeys []VoxelCoords) Plane {
	return &voxelPlane{normal, center, offset, points, voxelKeys}
}

func (p *voxelPlane) Normal() Vec3 {
	return Vec3(p.normal)
}

func (p *voxelPlane) Center() Vec3 {
	return Vec3(p.center)
}

func (p *voxelPlane) Offset() float64 {
	return p.offset
}

func (p *voxelPlane) PointCloud() (PointCloud, error) {
	pc := New()
	if p.points == nil {
		return nil, errors.New("No points in plane to turn into point cloud")
	}
	for _, pt := range p.points {
		err := pc.Set(pt)
		if err != nil {
			return nil, err
		}
	}
	return pc, nil
}

// Equation return the coefficients of the plane equation as a 4-slice of floats
func (p *voxelPlane) Equation() []float64 {
	equation := make([]float64, 4)
	equation[0] = p.normal.X
	equation[1] = p.normal.Y
	equation[2] = p.normal.Z
	equation[3] = p.offset
	return equation
}

// DistToPlane computes the distance between a point a plane with given normal vector and offset
func (p *voxelPlane) Distance(pt Vec3) float64 {
	num := math.Abs(r3.Vector(pt).Dot(p.normal) + p.offset)
	denom := p.normal.Norm()
	d := 0.
	if denom > 0.0001 {
		d = num / denom
	}
	return d
}

// Voxel is the structure to store data relevant to Voxel operations in point clouds
type Voxel struct {
	Key             VoxelCoords
	Label           int
	Points          []Point
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
		Points:          make([]Point, 0),
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
	p := NewBasicPoint(pt.X, pt.Y, pt.Z)
	return &Voxel{
		Key:             coords,
		Label:           0,
		Points:          []Point{p},
		Center:          r3.Vector{},
		Normal:          r3.Vector{},
		Offset:          0,
		Residual:        0,
		Weight:          0,
		SortedWeightIdx: 0,
		PointLabels:     nil,
	}
}

// Positions gets the positions of the points inside the voxel
func (v1 *Voxel) Positions() []r3.Vector {
	positions := make([]r3.Vector, len(v1.Points))
	for i, pt := range v1.Points {
		positions[i] = r3.Vector(pt.Position())
	}
	return positions
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
	for _, pt := range v1.Positions() {
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
	return NewPlaneFromVoxel(v1.Normal, v1.Center, v1.Offset, v1.Points, keys)
}

// VoxelSlice is a slice that contains Voxels
type VoxelSlice []*Voxel

func (d VoxelSlice) ToPointCloud() (PointCloud, error) {
	cloud := New()
	for _, vox := range d {
		for _, pt := range vox.Points {
			err := cloud.Set(pt)
			if err != nil {
				return nil, err
			}
		}
	}
	return cloud, nil
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

// VoxelGrid contains the sparse grid of Voxels of a point cloud
type VoxelGrid struct {
	Voxels    map[VoxelCoords]*Voxel
	maxLabel  int
	voxelSize float64
	lam       float64
}

// NewVoxelGrid returns a pointer to a VoxelGrid with a (0,0,0) Voxel
func NewVoxelGrid(voxelSize, lam float64) *VoxelGrid {
	voxelMap := make(map[VoxelCoords]*Voxel)
	coords := VoxelCoords{
		I: 0,
		J: 0,
		K: 0,
	}
	voxelMap[coords] = NewVoxel(coords)

	return &VoxelGrid{
		Voxels:    voxelMap,
		maxLabel:  0,
		voxelSize: voxelSize,
		lam:       lam,
	}
}

// VoxelSize is the side length of the voxels in the VoxelGrid
func (vg *VoxelGrid) VoxelSize() float64 {
	return vg.voxelSize
}

// Lambda is the clustering parameter for making voxel planes
func (vg *VoxelGrid) Lambda() float64 {
	return vg.lam
}

// VoxelHistogram creates useful plots for determining the parameters of the voxel grid when calibrating a new sensor.
// Hisotgrams of the number of points in each voxel, the weights of each voxel, and the plane residuals.
func (vg *VoxelGrid) VoxelHistogram(w, h int, name string) (image.Image, error) {
	var hist *hbook.H1D
	p := hplot.New()
	if name == "points" {
		p.Title.Text = "Points in Voxel"
		p.X.Label.Text = "Pts in Voxel"
		p.Y.Label.Text = "N Voxels"
		hist = hbook.NewH1D(25, 0, +25)
		for _, vox := range vg.Voxels {
			variable := float64(len(vox.Points))
			hist.Fill(variable, 1)
		}
	} else if name == "weights" {
		hist = hbook.NewH1D(40, 0, +1)
		p.Title.Text = "Weights of Voxel"
		p.X.Label.Text = "Voxel Weight"
		p.Y.Label.Text = "N Voxels"
		for _, vox := range vg.Voxels {
			variable := -9.0
			if len(vox.Points) > 5 {
				vox.Center = GetVoxelCenter(vox.Positions())
				vox.Normal = estimatePlaneNormalFromPoints(vox.Positions())
				vox.Offset = GetOffset(vox.Center, vox.Normal)
				vox.Residual = GetResidual(vox.Positions(), vox.GetPlane())
				variable = GetWeight(vox.Positions(), vg.lam, vox.Residual)
			}
			hist.Fill(variable, 1)
		}
	} else if name == "residuals" {
		hist = hbook.NewH1D(65, 0, +6.5)
		p.Title.Text = "Residual of Voxel"
		p.X.Label.Text = "Voxel Residuals"
		p.Y.Label.Text = "N Voxels"
		for _, vox := range vg.Voxels {
			variable := -999.
			if len(vox.Points) > 5 {
				vox.Center = GetVoxelCenter(vox.Positions())
				vox.Normal = estimatePlaneNormalFromPoints(vox.Positions())
				vox.Offset = GetOffset(vox.Center, vox.Normal)
				vox.Residual = GetResidual(vox.Positions(), vox.GetPlane())
				variable = vox.Residual
			}
			hist.Fill(variable, 1)
		}
	} else {
		return nil, fmt.Errorf("%s not a plottable variable", name)
	}

	// Create a histogram of our values
	hp := hplot.NewH1D(hist)
	hp.Infos.Style = hplot.HInfoSummary
	p.Add(hp)

	width, err := vecg.ParseLength(fmt.Sprintf("%dpt", w))
	if err != nil {
		return nil, err
	}
	height, err := vecg.ParseLength(fmt.Sprintf("%dpt", h))
	if err != nil {
		return nil, err
	}
	imgByte, err := hplot.Show(p, width, height, "png")
	if err != nil {
		return nil, err
	}
	img, _, err := image.Decode(bytes.NewReader(imgByte))
	if err != nil {
		return nil, err
	}
	return img, nil
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

// GetNNearestVoxels gets voxels around a grid coordinate that are N units away in each dimension.
func (vg VoxelGrid) GetNNearestVoxels(v *Voxel, n uint) []VoxelCoords {
	I, J, K := v.Key.I, v.Key.J, v.Key.K
	N := int64(n)
	is := []int64{I - N, I, I + N}
	js := []int64{J - N, J, J + N}
	ks := []int64{K - N, K, K + N}
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
			ptValue := pt.SetValue(label)
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
	voxelMap := NewVoxelGrid(voxelSize, lam)
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
				Points:          []Point{p},
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
			vox.Points = append(vox.Points, p)
		}
		return true
	})

	// All points are now assigned to a voxel in the voxel grid
	// Compute voxel attributes
	for k, vox := range voxelMap.Voxels {
		// Voxel must have enough point to make relevant computations
		vox.Key = k
		center := GetVoxelCenter(vox.Positions())
		vox.Center.X = center.X
		vox.Center.Y = center.Y
		vox.Center.Z = center.Z

		// below 5 points, normal and center estimation are not relevant
		if len(vox.Points) > 5 {
			vox.Normal = estimatePlaneNormalFromPoints(vox.Positions())
			vox.Offset = GetOffset(vox.Center, vox.Normal)
			vox.Residual = GetResidual(vox.Positions(), vox.GetPlane())
			vox.Weight = GetWeight(vox.Positions(), lam, vox.Residual)
		}

	}
	return voxelMap
}
