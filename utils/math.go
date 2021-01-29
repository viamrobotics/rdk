package utils

import "math"

func DegToRad(degrees float64) float64 {
	return degrees * math.Pi / 180
}

func RadToDeg(radians float64) float64 {
	return radians * 180 / math.Pi
}
