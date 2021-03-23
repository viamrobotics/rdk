package pointcloud

import (
	"fmt"
	"math"

	"github.com/edaniels/golog"
	"gonum.org/v1/gonum/mat"
)

type key Vec3

type PointCloud struct {
	points     map[key]Point
	hasColor   bool
	hasValue   bool
	minX, maxX int
	minY, maxY int
	minZ, maxZ int
	logger     golog.Logger
}

func New(logger golog.Logger) *PointCloud {
	return &PointCloud{
		points: map[key]Point{},
		minX:   math.MaxInt64,
		minY:   math.MaxInt64,
		minZ:   math.MaxInt64,
		maxX:   math.MinInt64,
		maxY:   math.MinInt64,
		maxZ:   math.MinInt64,
		logger: logger,
	}
}

func (cloud *PointCloud) Size() int {
	return len(cloud.points)
}

func (cloud *PointCloud) At(x, y, z int) Point {
	return cloud.points[key{x, y, z}]
}

const (
	maxExactFloat64Integer = 1 << 53
	minExactFloat64Integer = -maxExactFloat64Integer
)

func newOutOfRangeErr(dim string, val int) error {
	return fmt.Errorf("%s component (%d) is out of range [%d,%d]", dim, val, minExactFloat64Integer, maxExactFloat64Integer)
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
	if v.X > maxExactFloat64Integer || v.X < minExactFloat64Integer {
		return newOutOfRangeErr("x", v.X)
	}
	if v.Y > maxExactFloat64Integer || v.Y < minExactFloat64Integer {
		return newOutOfRangeErr("y", v.Y)
	}
	if v.Z > maxExactFloat64Integer || v.Z < minExactFloat64Integer {
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

func (cloud *PointCloud) Unset(x, y, z int) {
	delete(cloud.points, key{x, y, z})
}

func (cloud *PointCloud) Iterate(fn func(p Point) bool) {
	for _, p := range cloud.points {
		if cont := fn(p); !cont {
			return
		}
	}
}

func newDensePivotFromCloud(cloud *PointCloud, dim int, idx int) (*mat.Dense, error) {
	size := cloud.Size()
	m := mat.NewDense(2, size, nil)
	var data []int
	c := 0
	var err error
	cloud.Iterate(func(p Point) bool {
		v := p.Position()
		var i, j, k int
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
		m.Set(0, c, float64(i))
		m.Set(1, c, float64(j))
		data = append(data, i, j)
		c++
		return true
	})
	return m, err
}

// TODO(erd): intermediate, lazy structure that is not dense floats?
func (cloud *PointCloud) DenseZ(zIdx int) (*mat.Dense, error) {
	return newDensePivotFromCloud(cloud, 2, zIdx)
}
