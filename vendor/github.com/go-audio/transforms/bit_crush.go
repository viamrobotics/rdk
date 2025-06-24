package transforms

import (
	"math"

	"github.com/go-audio/audio"
)

var (
	crusherStepSize  = 0.000001
	CrusherMinFactor = 1.0
	CrusherMaxFactor = 2097152.0
)

// BitCrush reduces the resolution of the sample to the target bit depth
// Note that bit crusher effects are usually made of this feature + a decimator
func BitCrush(buf *audio.FloatBuffer, factor float64) {
	stepSize := crusherStepSize * factor
	for i := 0; i < len(buf.Data); i++ {
		frac, exp := math.Frexp(buf.Data[i])
		frac = signum(frac) * math.Floor(math.Abs(frac)/stepSize+0.5) * stepSize
		buf.Data[i] = math.Ldexp(frac, exp)
	}
}

func signum(v float64) float64 {
	if v >= 0.0 {
		return 1.0
	}
	return -1.0
}
