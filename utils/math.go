// Package utils contains all utility functions that currently have no better home than here.
// Consider moving them to go.viam.com/utils.
package utils

import (
	"math"
	"math/rand"
	"sort"

	"github.com/golang/geo/r3"
)

// DegToRad converts degrees to radians.
func DegToRad(degrees float64) float64 {
	return degrees * math.Pi / 180
}

// RadToDeg converts radians to degrees.
func RadToDeg(radians float64) float64 {
	return radians * 180 / math.Pi
}

// AngleDiffDeg returns the closest difference from the two given
// angles. The arguments are commutative.
func AngleDiffDeg(a1, a2 float64) float64 {
	return float64(180) - math.Abs(math.Abs(a1-a2)-float64(180))
}

// AntiCWDeg flips the given degrees as if you were to start at 0 and
// go counter-clockwise or vice versa.
func AntiCWDeg(deg float64) float64 {
	return math.Mod(float64(360)-deg, 360)
}

// ModAngDeg returns the given angle modulus 360 and resolves
// any negativity.
func ModAngDeg(ang float64) float64 {
	return math.Mod(math.Mod((ang), 360)+360, 360)
}

// Median returns the median value of the given values. If there
// are no values, NaN is returned.
func Median(values ...float64) float64 {
	if len(values) == 0 {
		return math.NaN()
	}
	sort.Float64s(values)

	return values[int(math.Floor(float64(len(values))/2))]
}

// AbsInt returns the absolute value of the given value.
func AbsInt(n int) int {
	if n < 0 {
		return -1 * n
	}
	return n
}

// AbsInt64 returns the absolute value of the given value.
func AbsInt64(n int64) int64 {
	if n < 0 {
		return -1 * n
	}
	return n
}

// MaxInt returns the maximum of two values.
func MaxInt(a, b int) int {
	if a < b {
		return b
	}
	return a
}

// MinInt returns the minimum of two values.
func MinInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// MaxUint8 returns the maximum of two values.
func MaxUint8(a, b uint8) uint8 {
	if a < b {
		return b
	}
	return a
}

// MinUint8 returns the minimum of two values.
func MinUint8(a, b uint8) uint8 {
	if a < b {
		return a
	}
	return b
}

const cubeRootExp = 1.0 / 3.0

// CubeRoot returns the cube root of the given value.
func CubeRoot(x float64) float64 {
	return math.Pow(x, cubeRootExp)
}

// Square returns the square of the given value.
// Math.pow( x, 2 ) is slow, this is faster.
func Square(n float64) float64 {
	return n * n
}

// SquareInt returns the square of the given value.
// Math.pow( x, 2 ) is slow, this is faster.
func SquareInt(n int) int {
	return n * n
}

// ScaleByPct scales a max number by a floating point percentage between two bounds [0, n].
func ScaleByPct(n int, pct float64) int {
	scaled := int(float64(n) * pct)
	if scaled < 0 {
		scaled = 0
	} else if scaled > n {
		scaled = n
	}
	return scaled
}

// SampleRandomIntRange samples a random integer within a range given by [min, max]
// using the given rand.Rand.
func SampleRandomIntRange(min, max int, r *rand.Rand) int {
	return r.Intn(max-min+1) + min
}

// Float64AlmostEqual compares two float64s and returns if the difference between them is less than epsilon.
func Float64AlmostEqual(a, b, epsilon float64) bool {
	return (a-b) < epsilon && (b-a) < epsilon
}

// R3VectorAlmostEqual compares two r3.Vector objects and returns if the all elementwise differences are less than epsilon.
func R3VectorAlmostEqual(a, b r3.Vector, epsilon float64) bool {
	return math.Abs(a.X-b.X) < epsilon && math.Abs(a.Y-b.Y) < epsilon && math.Abs(a.Z-b.Z) < epsilon
}

// Clamp returns min if value is lesser than min, max if value is greater them max or value if the input value is
// between min and max.
func Clamp(value float64, min float64, max float64) float64 {
	if value < min {
		return min
	} else if value > max {
		return max
	}
	return value
}
