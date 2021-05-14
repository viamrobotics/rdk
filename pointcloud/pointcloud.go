// Package pointcloud defines a point cloud and provides an implementation for one.
//
// Its implementation is dictionary based is not yet efficient. The current focus is
// to make it useful and as such the API is experimental and subject to change
// considerably.
package pointcloud

import (
	"io"
	"math"

	"github.com/go-errors/errors"
	"gonum.org/v1/gonum/mat"

	"go.viam.com/core/utils"
)

// key is the map key used within the point cloud implementation. That is
// we index points by their positions. This is problematic for points that
// can mutate their own position outside the ownership of the cloud.
type key Vec3

// PointCloud is a general purpose container of points. It does not
// dictate whether or not the cloud is sparse or dense. The current
// basic implementation is sparse however.
type PointCloud interface {
	// Size returns the number of points in the cloud.
	Size() int

	// HasColor returns whether or not the cloud consists of points with color.
	HasColor() bool

	// HasValue returns whether or not the cloud consists of points with user data value.
	HasValue() bool

	// MinX returns the minimum x coordinate of all points in the cloud.
	MinX() float64

	// MaxY returns the maximum y coordinate of all points in the cloud.
	MaxX() float64

	// MinY returns the minimum y coordinate of all points in the cloud.
	MinY() float64

	// MaxY returns the maximum y coordinate of all points in the cloud.
	MaxY() float64

	// MinZ returns the minimum z coordinate of all points in the cloud.
	MinZ() float64

	// MaxZ returns the maximum z coordinate of all points in the cloud.
	MaxZ() float64

	// Set places the given point in the cloud.
	Set(p Point) error

	// Unset removes a point from the cloud exists at the given position.
	// If the point does not exist, this does nothing.
	Unset(x, y, z float64)

	// At returns the point in the cloud at the given position, if it exists;
	// returns nil otherwise.
	At(x, y, z float64) Point

	// Iterate iterates over all points in the cloud and calls the given
	// function for each point. If the supplied function returns false,
	// iteration will stop after the function returns.
	Iterate(fn func(p Point) bool)

	// WriteToFile writes the point cloud in LAS format to the given file.
	WriteToFile(fn string) error

	// ToPCD writes the point cloud to a PCD backed by the given writer. The
	// caller is responsible for closing it.
	ToPCD(out io.Writer) error

	// DenseZ returns a matrix representing an X,Y plane based off a Z coordinate.
	DenseZ(zIdx float64) (*mat.Dense, error)

	// ToVec2Matrix converts the point cloud into a matrix representing a list
	// of the points, not a three-dimensional dense matrix. As this is two-dimensional,
	// it is based off the X,Y plane at the 0 Z coordinate making this useful for only 2D
	// point clouds.
	ToVec2Matrix() (*utils.Vec2Matrix, error)
}

// basicPointCloud is the basic implementation of the PointCloud interface backed by
// a map of points keyed by position.
type basicPointCloud struct {
	points     map[key]Point
	hasColor   bool
	hasValue   bool
	minX, maxX float64
	minY, maxY float64
	minZ, maxZ float64
}

// New returns an empty PointCloud backed by a basicPointCloud.
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

// With 64-bit floating point numbers, you get about 16 decimal digits of precision.
// To guarantee at least 6 decimal places of precision past 0, Abs(x) cannot be greater than 2^33 - 1.
const (
	maxPreciseFloat64 = 8589934591
	minPreciseFloat64 = -8589934591
)

// newOutOfRangeErr returns an error informing that a value is numerically out of range to
// be stored precisely.
func newOutOfRangeErr(dim string, val float64) error {
	return errors.Errorf("%s component (%v) is out of range [%v,%v]", dim, val, minPreciseFloat64, maxPreciseFloat64)
}

// Set validates that the point can be precisely stored before setting it in the cloud.
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
			err = errors.Errorf("unknown dim %d", dim)
			return false
		}
		if k != idx {
			return true
		}
		// floating point lossiness validated/warned from set/load
		m.Set(0, c, i)
		m.Set(1, c, j)
		data = append(data, i, j)
		c++
		return true
	})
	return m, err
}

func (cloud *basicPointCloud) DenseZ(zIdx float64) (*mat.Dense, error) {
	// Note(erd): would be nice if this was lazy and not dense
	return newDensePivotFromCloud(cloud, 2, zIdx)
}
