package pointcloud

import (
	"fmt"
	"io"
	"math"

	"go.viam.com/robotcore/utils"
	"gonum.org/v1/gonum/mat"
)

type key Vec3

type PointCloud interface {
	Size() int
	HasColor() bool
	HasValue() bool
	MinX() float64
	MaxX() float64
	MinY() float64
	MaxY() float64
	MinZ() float64
	MaxZ() float64

	// point setting and getting methods

	Set(p Point) error
	Unset(x, y, z float64)
	At(x, y, z float64) Point
	Iterate(fn func(p Point) bool)

	// utils

	// Writes to a LAS file
	WriteToFile(fn string) error
	ToPCD(out io.Writer) error
	DenseZ(zIdx float64) (*mat.Dense, error)
	ToVec2Matrix() (*utils.Vec2Matrix, error)
}

// basicPointCloud is the basic implementation of the PointCloud interface
type basicPointCloud struct {
	points     map[key]Point
	hasColor   bool
	hasValue   bool
	minX, maxX float64
	minY, maxY float64
	minZ, maxZ float64
}

func New() PointCloud {
	return &basicPointCloud{
		points: map[key]Point{},
		minX:   math.MaxFloat64,
		minY:   math.MaxFloat64,
		minZ:   math.MaxFloat64,
		maxX:   -math.MaxFloat64,
		maxY:   -math.MaxFloat64,
		maxZ:   -math.MaxFloat64,
	}
}

func (cloud *basicPointCloud) Size() int {
	return len(cloud.points)
}

func (cloud *basicPointCloud) HasColor() bool {
	return cloud.hasColor
}

func (cloud *basicPointCloud) HasValue() bool {
	return cloud.hasValue
}

func (cloud *basicPointCloud) MinX() float64 {
	return cloud.minX
}

func (cloud *basicPointCloud) MaxX() float64 {
	return cloud.maxX
}

func (cloud *basicPointCloud) MinY() float64 {
	return cloud.minY
}

func (cloud *basicPointCloud) MaxY() float64 {
	return cloud.maxY
}

func (cloud *basicPointCloud) MinZ() float64 {
	return cloud.minZ
}

func (cloud *basicPointCloud) MaxZ() float64 {
	return cloud.maxZ
}

func (cloud *basicPointCloud) At(x, y, z float64) Point {
	return cloud.points[key{x, y, z}]
}

// With 64bit floating point numbers, you get about 16 decimal digits of precision.
// To guarantee at least 6 decimal places of precision past 0, Abs(x) cannot be greater than 2^33 - 1
const (
	maxPreciseFloat64 = 8589934591
	minPreciseFloat64 = -8589934591
)

func newOutOfRangeErr(dim string, val float64) error {
	return fmt.Errorf("%s component (%v) is out of range [%v,%v]", dim, val, minPreciseFloat64, maxPreciseFloat64)
}

func (cloud *basicPointCloud) Set(p Point) error {
	cloud.points[key(p.Position())] = p
	if p.HasColor() {
		cloud.hasColor = true
	}
	if p.HasValue() {
		cloud.hasValue = true
	}
	v := p.Position()
	if v.X > maxPreciseFloat64 || v.X < minPreciseFloat64 {
		return newOutOfRangeErr("x", v.X)
	}
	if v.Y > maxPreciseFloat64 || v.Y < minPreciseFloat64 {
		return newOutOfRangeErr("y", v.Y)
	}
	if v.Z > maxPreciseFloat64 || v.Z < minPreciseFloat64 {
		return newOutOfRangeErr("z", v.Z)
	}
	if v.X > cloud.maxX {
		cloud.maxX = v.X
	}
	if v.Y > cloud.maxY {
		cloud.maxY = v.Y
	}
	if v.Z > cloud.maxZ {
		cloud.maxZ = v.Z
	}

	if v.X < cloud.minX {
		cloud.minX = v.X
	}
	if v.Y < cloud.minY {
		cloud.minY = v.Y
	}
	if v.Z < cloud.minZ {
		cloud.minZ = v.Z
	}
	return nil
}

func (cloud *basicPointCloud) Unset(x, y, z float64) {
	delete(cloud.points, key{x, y, z})
}

func (cloud *basicPointCloud) Iterate(fn func(p Point) bool) {
	for _, p := range cloud.points {
		if cont := fn(p); !cont {
			return
		}
	}
}

func newDensePivotFromCloud(cloud PointCloud, dim int, idx float64) (*mat.Dense, error) {
	size := cloud.Size()
	m := mat.NewDense(2, size, nil)
	var data []float64
	c := 0
	var err error
	cloud.Iterate(func(p Point) bool {
		v := p.Position()
		var i, j, k float64
		switch dim {
		case 0:
			i = v.Y
			j = v.Z
			k = v.X
		case 1:
			i = v.X
			j = v.Z
			k = v.Y
		case 2:
			i = v.X
			j = v.Y
			k = v.Z
		default:
			err = fmt.Errorf("unknown dim %d", dim)
			return false
		}
		if k != idx {
			return true
		}
		// floating point losiness validated/warned from set/load
		m.Set(0, c, i)
		m.Set(1, c, j)
		data = append(data, i, j)
		c++
		return true
	})
	return m, err
}

func (cloud *basicPointCloud) DenseZ(zIdx float64) (*mat.Dense, error) {
	// would be nice if this was lazy and not dense
	return newDensePivotFromCloud(cloud, 2, zIdx)
}
