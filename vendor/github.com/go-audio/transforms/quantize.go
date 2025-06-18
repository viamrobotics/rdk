package transforms

import (
	"math"

	"github.com/go-audio/audio"
)

// Quantize quantizes the audio signal to match the target bitDepth
func Quantize(buf *audio.FloatBuffer, bitDepth int) {
	if buf == nil {
		return
	}
	max := math.Pow(2, float64(bitDepth)) - 1

	bufLen := len(buf.Data)
	for i := 0; i < bufLen; i++ {
		buf.Data[i] = round((buf.Data[i]+1)*max)/max - 1.0
	}
}

func round(f float64) float64 {
	if f > 0.0 {
		return math.Floor(f + 0.5)
	}
	return math.Ceil(f - 0.5)
}
