package transforms

import (
	"math"

	"github.com/go-audio/audio"
)

// NormalizeMax sets the max value to 1 and normalize the rest of the data.
func NormalizeMax(buf *audio.FloatBuffer) {
	if buf == nil {
		return
	}
	max := 0.0

	for i := 0; i < len(buf.Data); i++ {
		if math.Abs(buf.Data[i]) > max {
			max = math.Abs(buf.Data[i])
		}
	}

	if max != 0.0 {
		for i := 0; i < len(buf.Data); i++ {
			buf.Data[i] /= max
		}
	}
}
