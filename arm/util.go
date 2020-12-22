package arm

import (
	"math"
)

func convertRtoD(r float64) float64 {
	return r * 180 / math.Pi
}
