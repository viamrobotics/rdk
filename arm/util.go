package arm

import (
	"math"
)

func convertRtoD(r float64) float64 {
	return r * 180 / math.Pi
}

func convertDtoR(r float64) float64 {
	return math.Pi * (r / 180)
}


