package pointcloud

import (
	"fmt"
	"math"

	"gonum.org/v1/gonum/mat"
)

type key Vec3

type PointCloud struct {
	points     map[key]Point
	hasColor   bool
	hasValue   bool
	minX, maxX float64
	minY, maxY float64
	minZ, maxZ float64
}

func New() *PointCloud {
	return &PointCloud{
		points: map[key]Point{},
		minX:   math.MaxFloat64,
		minY:   math.MaxFloat64,
		minZ:   math.MaxFloat64,
		maxX:   -math.MaxFloat64,
		maxY:   -math.MaxFloat64,
		maxZ:   -math.MaxFloat64,
	}
}

func (cloud *PointCloud) Size() int {
	return len(cloud.points)
}

func (cloud *PointCloud) AtInt(x, y, z int) Point {
	return cloud.At(float64(x), float64(y), float64(z))
}

func (cloud *PointCloud) At(x, y, z float64) Point {
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

func (cloud *PointCloud) Set(p Point) error {
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

func (cloud *PointCloud) UnsetInt(x, y, z int) {
	cloud.Unset(float64(x), float64(y), float64(z))
}

func (cloud *PointCloud) Unset(x, y, z float64) {
	delete(cloud.points, key{x, y, z})
}

func (cloud *PointCloud) Iterate(fn func(p Point) bool) {
	for _, p := range cloud.points {
		if cont := fn(p); !cont {
			return
		}
	}
}

func newDensePivotFromCloud(cloud *PointCloud, dim int, idx float64) (*mat.Dense, error) {
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

func (cloud *PointCloud) DenseZ(zIdx float64) (*mat.Dense, error) {
	// would be nice if this was lazy and not dense
	return newDensePivotFromCloud(cloud, 2, zIdx)
}
