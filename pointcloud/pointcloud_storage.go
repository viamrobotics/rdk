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
	IsOrdered() bool
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
