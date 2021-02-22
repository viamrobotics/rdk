package utils

import (
	"math"
	"sort"
)

func DegToRad(degrees float64) float64 {
	return degrees * math.Pi / 180
}

func RadToDeg(radians float64) float64 {
	return radians * 180 / math.Pi
}

// AngleDiffDeg returns the closest difference from the two given
// angles. The arguments are commutative.
func AngleDiffDeg(a1, a2 float64) float64 {
	return float64(180) - math.Abs(math.Abs(a1-a2)-float64(180))
}

func AntiCWDeg(deg float64) float64 {
	return math.Mod(float64(360)-deg, 360)
}

func Median(values ...float64) float64 {
	if len(values) == 0 {
		return math.NaN()
	}
	sort.Float64s(values)

	return values[int(math.Floor(float64(len(values))/2))]
}

func AbsInt(n int) int {
	if n < 0 {
		return -1 * n
	}
	return n
}

func MaxInt(a, b int) int {
	if a < b {
		return b
	}
	return a
}

func MinInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// Math.pow( x, 2 ) is slow, this is faster
func Square(n float64) float64 {
	return n * n
}

// Math.pow( x, 2 ) is slow, this is faster
func SquareInt(n int) int {
	return n * n
}
