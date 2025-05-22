package transforms

import "github.com/go-audio/audio"

// Gain applies the multiplier to the passed buffer.
// A multipler of 1 would increase the amplitude of the signal by 100%
// while a multiplier of 0 would not do anything.
// Note that this is a very very naive implementation and we will more
// than add a more useful DB gain transform.
func Gain(buf *audio.FloatBuffer, multiplier float64) error {
	if buf == nil {
		return audio.ErrInvalidBuffer
	}

	for i := 0; i < len(buf.Data); i++ {
		buf.Data[i] *= multiplier
	}

	return nil
}
