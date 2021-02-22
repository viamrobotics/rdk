package pc

import (
	"fmt"
	"math"

	"gonum.org/v1/gonum/mat"
)

type Vec3 struct {
	X, Y, Z float64
}

type key Vec3

// TODO(erd): LAS stores 4 byte longs for x,y,z
// so float64 doesn't really work here
type PointCloud struct {
	points     map[key]Point
	hasColor   bool
	hasValue   bool
	minX, maxX float64
	minY, maxY float64
	minZ, maxZ float64
}

func NewPointCloud() *PointCloud {
	return &PointCloud{
		points: map[key]Point{},
		minX:   math.MaxInt64,
		minY:   math.MaxInt64,
		minZ:   math.MaxInt64,
		maxX:   math.MinInt64,
		maxY:   math.MinInt64,
		maxZ:   math.MinInt64,
	}
}

func (cloud *PointCloud) Size() int {
	return len(cloud.points)
}

func (cloud *PointCloud) At(x, y, z float64) Point {
	return cloud.points[key{x, y, z}]
}

func (cloud *PointCloud) Set(p Point) {
	cloud.points[key(p.Position())] = p
	if ok, _ := IsColored(p); ok {
		cloud.hasColor = true
	}
	if ok, _ := IsFloat(p); ok {
		cloud.hasValue = true
	}
	v := p.Position()
	if v.X < 0 {
		panic(v.X)
	}
	if v.Y < 0 {
		panic(v.X)
	}
	if v.Z < 0 {
		panic(v.X)
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

func newDensePivotFromCloud(cloud *PointCloud, dim int, idx float64) *mat.Dense {
	size := cloud.Size()
	m := mat.NewDense(2, size, nil)
	var data []float64
	c := 0
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
			panic(fmt.Errorf("unknown dim %d", dim))
		}
		if k != idx {
			return true
		}
		m.Set(0, c, i)
		m.Set(1, c, j)
		data = append(data, i, j)
		c++
		return true
	})
	return m
}

// TODO(erd): intermediate, lazy structure that is not dense floats?
func (cloud *PointCloud) DenseZ(zIdx float64) *mat.Dense {
	return newDensePivotFromCloud(cloud, 2, zIdx)
}
