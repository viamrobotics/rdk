// Package utils contains all utility functions that currently have no better home than here.
// Consider moving them to go.viam.com/utils.
package utils

import (
	"encoding/binary"
	"math"
	"math/rand"
	"sort"

	"gonum.org/v1/gonum/stat/distuv"
)

// MetersToMM converts a value in meters to a value in mm
func MetersToMM(m float64) float64 {
	return m * 1000
}

// MMToMeters converts a value in mm to a value in meters
func MMToMeters(m float64) float64 {
	return m * 1000
}

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

// Clamp returns min if value is lesser than min, max if value is greater them max or value if the input value is
// between min and max.
func Clamp(value, min, max float64) float64 {
	if value < min {
		return min
	} else if value > max {
		return max
	}
	return value
}

// CycleIntSliceByN cycles the list to the right by n steps.
func CycleIntSliceByN(s []int, n int) []int {
	n %= len(s)
	res := make([]int, 0, len(s))
	res = append(res, s[n:]...)
	res = append(res, s[:n]...)
	return res
}

// SampleNRegularlySpaced returns the same set of evenly divided numbers every time, and is mostly used for testing purposes.
func SampleNRegularlySpaced(n int, vMin, vMax float64) []int {
	if vMax < vMin {
		panic("vMax cannot be less than vMin")
	}
	length := vMax - vMin
	step := length / float64(n)
	result := make([]int, n)
	add := 0.0
	for i := range result {
		result[i] = int(vMin) + int(math.Round(add))
		add += step
	}
	return result
}

// SampleNIntegersNormal samples n integers from normal distribution centered around (vMax+vMin) / 2
// and in range [vMin, vMax].
func SampleNIntegersNormal(n int, vMin, vMax float64) []int {
	z := make([]int, n)
	// get normal distribution centered on (vMax+vMin) / 2 and whose sampled are mostly in [vMin, vMax] (var=0.1)
	mean := (vMax + vMin) / 2
	dist := distuv.Normal{
		Mu:    mean,
		Sigma: (vMax - vMin) * 0.4472,
	}
	for i := range z {
		val := math.Round(dist.Rand())
		for val < vMin || val > vMax {
			val = math.Round(dist.Rand())
		}
		z[i] = int(val)
	}

	return z
}

// SampleNIntegersUniform samples n integers uniformly in [vMin, vMax].
func SampleNIntegersUniform(n int, vMin, vMax float64) []int {
	z := make([]int, n)
	// get uniform distribution on [vMin, vMax]
	dist := distuv.Uniform{
		Min: vMin,
		Max: vMax,
	}
	for i := range z {
		val := math.Round(dist.Rand())
		for val < vMin || val > vMax {
			val = math.Round(dist.Rand())
		}
		z[i] = int(val)
	}

	return z
}

// BytesFromFloat64LE converts a float64 to an array of bytes ordered in little-endian.
func BytesFromFloat64LE(v float64) []byte {
	var b [8]byte
	binary.LittleEndian.PutUint64(b[:], math.Float64bits(v))
	return b[:]
}

// BytesFromFloat32LE converts a float32 to an array of bytes ordered in little-endian.
func BytesFromFloat32LE(v float32) []byte {
	var b [4]byte
	binary.LittleEndian.PutUint32(b[:], math.Float32bits(v))
	return b[:]
}

// BytesFromFloat64BE converts a float64 to an array of bytes ordered in big-endian.
func BytesFromFloat64BE(v float64) []byte {
	var b [8]byte
	binary.BigEndian.PutUint64(b[:], math.Float64bits(v))
	return b[:]
}

// BytesFromFloat32BE converts a float32 to an array of bytes ordered in big-endian.
func BytesFromFloat32BE(v float32) []byte {
	var b [4]byte
	binary.BigEndian.PutUint32(b[:], math.Float32bits(v))
	return b[:]
}

// BytesFromUint32LE converts a uint32 to an array of bytes ordered in little-endian.
func BytesFromUint32LE(v uint32) []byte {
	var b [4]byte
	binary.LittleEndian.PutUint32(b[:], v)
	return b[:]
}

// BytesFromUint32BE converts a uint32 to an array of bytes ordered in big-endian.
func BytesFromUint32BE(v uint32) []byte {
	var b [4]byte
	binary.BigEndian.PutUint32(b[:], v)
	return b[:]
}

// Float32FromBytesLE converts an array of byte ordered in little-endian to a float32.
func Float32FromBytesLE(bytes []byte) float32 {
	bits := binary.LittleEndian.Uint32(bytes)
	float := math.Float32frombits(bits)
	return float
}

// Float64FromBytesLE converts an array of byte ordered in little-endian to a float64.
func Float64FromBytesLE(bytes []byte) float64 {
	bits := binary.LittleEndian.Uint64(bytes)
	float := math.Float64frombits(bits)
	return float
}

// Float32FromBytesBE converts an array of byte ordered in big-endian to a float32.
func Float32FromBytesBE(bytes []byte) float32 {
	bits := binary.BigEndian.Uint32(bytes)
	float := math.Float32frombits(bits)
	return float
}

// Float64FromBytesBE converts an array of byte ordered in big-endian to a float64.
func Float64FromBytesBE(bytes []byte) float64 {
	bits := binary.BigEndian.Uint64(bytes)
	float := math.Float64frombits(bits)
	return float
}

// Uint32FromBytesLE converts an array of bytes ordered in little-endian to a uint32.
func Uint32FromBytesLE(bytes []byte) uint32 {
	return binary.LittleEndian.Uint32(bytes)
}

// Uint32FromBytesBE converts an array of bytes ordered in big-endian to a uint32.
func Uint32FromBytesBE(bytes []byte) uint32 {
	return binary.BigEndian.Uint32(bytes)
}

// Int16FromBytesLE converts an array of bytes ordered in little-endian to a (signed) int16.
func Int16FromBytesLE(bytes []byte) int16 {
	return int16(binary.LittleEndian.Uint16(bytes))
}

// Int16FromBytesBE converts an array of bytes ordered in big-endian to a (signed) int16.
func Int16FromBytesBE(bytes []byte) int16 {
	return int16(binary.BigEndian.Uint16(bytes))
}
