package utils

import "math"

func DegToRad(degrees float64) float64 {
	return degrees * math.Pi / 180
}

func RadToDeg(radians float64) float64 {
	return radians * 180 / math.Pi
}

func AngleDiffDeg(a1, a2 float64) float64 {
	return float64(180) - math.Abs(math.Abs(a1-a2)-float64(180))
}

func AntiCWDeg(deg float64) float64 {
	return math.Mod(float64(360)-deg, 360)
}

func AverageAngleDeg(angles ...float64) float64 {
	sumSin := 0.0
	sumCos := 0.0
	for _, ang := range angles {
		angleRad := DegToRad(ang)
		sumSin += math.Sin(angleRad)
		sumCos += math.Cos(angleRad)
	}
	ret := RadToDeg(math.Atan2(sumSin, sumCos))
	if ret < 0 {
		ret = 360 + ret
	}
	return ret
}

func MedianAngleDeg(angles ...float64) float64 {
	rets := make([]float64, 0, len(angles))
	for _, ang := range angles {
		angleRad := DegToRad(ang)
		sin := math.Sin(angleRad)
		cos := math.Cos(angleRad)
		ret := RadToDeg(math.Atan2(sin, cos))
		if ret < 0 {
			ret = 360 + ret
		}
		rets = append(rets, ret)
	}

	return rets[int(math.Floor(float64(len(rets))/2))]
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
