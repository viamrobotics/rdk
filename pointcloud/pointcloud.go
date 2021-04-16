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
	minX, maxX float64
	minY, maxY float64
	minZ, maxZ float64
	logger     golog.Logger
}

func New(logger golog.Logger) *PointCloud {
	return &PointCloud{
		points: map[key]Point{},
		minX:   math.MaxFloat64,
		minY:   math.MaxFloat64,
		minZ:   math.MaxFloat64,
		maxX:   -math.MaxFloat64,
		maxY:   -math.MaxFloat64,
		maxZ:   -math.MaxFloat64,
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
	maxPreciseFloat64 = math.Pow(2, 33) - 1.0
	minPreciseFloat64 = -math.Pow(2, 33) + 1.0
)

func newOutOfRangeErr(dim string, val int) error {
	return fmt.Errorf("%s component (%d) is out of range [%d,%d]", dim, val, minPreciseFloat64, maxPreciseFloat64)
}

func outOfRange(x float64) bool {
	// to always have at least 6 decimal places of precision, Abs(x) cannot be greater than 2^33 - 1
	return ((math.Float64bits(x) >> 52) & 0b001111100000) != 0
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
	if outOfRange(v.X) {
		return newOutOfRangeErr("x", v.X)
	}
	if outOfRange(v.Y) {
		return newOutOfRangeErr("y", v.Y)
	}
	if outOfRange(v.Z) {
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

func (cloud *PointCloud) DenseZ(zIdx int) (*mat.Dense, error) {
	// would be nice if this was lazy and not dense
	return newDensePivotFromCloud(cloud, 2, zIdx)
}
