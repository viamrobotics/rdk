package utils

import (
	"math"
	"math/rand"
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

func ModAngDeg(ang float64) float64 {
	return math.Mod(math.Mod((ang), 360)+360, 360)
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

func AbsInt64(n int64) int64 {
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

func MaxUint8(a, b uint8) uint8 {
	if a < b {
		return b
	}
	return a
}

func MinUint8(a, b uint8) uint8 {
	if a < b {
		return a
	}
	return b
}

func CubeRoot(x float64) float64 {
	p := 1.0 / 3.0
	return math.Pow(x, p)
}

// Math.pow( x, 2 ) is slow, this is faster
func Square(n float64) float64 {
	return n * n
}

// Math.pow( x, 2 ) is slow, this is faster
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

// RayToUpwardCWCartesian returns coordinates based off of
// a coordinate system where the center is x,y=0,0 and
// zero degrees is pointing up. This is helpful for visualzing
// measurement devices that scan clockwise.
// ray is in degrees
// 0°   -  (0,increasing) // Up
// 90°  -  (increasing, 0) // Right
// 180° -  (0, decreasing) // Down
// 270° -  (decreasing,0) // Left
func RayToUpwardCWCartesian(angle, distance float64) (float64, float64) {
	angleRad := DegToRad(angle)
	x := distance * math.Sin(angleRad)
	y := distance * math.Cos(angleRad)
	return x, y
}

// SampleRandomInt samples a random integer within a range given by [min, max]
// using the given rand.Rand
func SampleRandomIntRange(min, max int, r *rand.Rand) int {
	return r.Intn(max-min+1) + min
}
