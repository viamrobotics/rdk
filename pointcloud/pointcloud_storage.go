package pointcloud

import (
	"github.com/golang/geo/r3"
	"github.com/pkg/errors"
)

type storage interface {
	Size() int
	Set(p r3.Vector, d Data) error
	At(x, y, z float64) (Data, bool)
	Iterate(numBatches, myBatch int, fn func(p r3.Vector, d Data) bool)
	EditSupported() bool
}

type mapStorage struct {
	points map[r3.Vector]Data
}

func (ms *mapStorage) Size() int {
	return len(ms.points)
}

// With 64-bit floating point numbers, you get about 16 decimal digits of precision.
// To guarantee at least 6 decimal places of precision past 0, Abs(x) cannot be greater than 2^33 - 1.
const (
	maxPreciseFloat64 = float64(8589934591)
	minPreciseFloat64 = float64(-8589934591)
)

// newOutOfRangeErr returns an error informing that a value is numerically out of range to
// be stored precisely.
func newOutOfRangeErr(dim string, val float64) error {
	return errors.Errorf("%s component (%v) is out of range [%v,%v]", dim, val, minPreciseFloat64, maxPreciseFloat64)
}

func (ms *mapStorage) Set(v r3.Vector, d Data) error {
	if v.X > maxPreciseFloat64 || v.X < minPreciseFloat64 {
		return newOutOfRangeErr("x", v.X)
	}
	if v.Y > maxPreciseFloat64 || v.Y < minPreciseFloat64 {
		return newOutOfRangeErr("y", v.Y)
	}
	if v.Z > maxPreciseFloat64 || v.Z < minPreciseFloat64 {
		return newOutOfRangeErr("z", v.Z)
	}
	ms.points[v] = d
	return nil
}

func (ms *mapStorage) At(x, y, z float64) (Data, bool) {
	d, found := ms.points[r3.Vector{x, y, z}]
	return d, found
}

func (ms *mapStorage) Iterate(numBatches, myBatch int, fn func(p r3.Vector, d Data) bool) {
	if numBatches > 0 && myBatch > 0 {
		// TODO(erh) finish me
		return
	}
	for p, d := range ms.points {
		if cont := fn(p, d); !cont {
			return
		}
	}
}

func (ms *mapStorage) EditSupported() bool {
	return true
}
