package transforms

import (
	"math"

	"github.com/go-audio/audio"
)

// PCMScale converts a buffer with audio content from -1 to 1 into
// the PCM scale based on the passed bitdepth.
// Note that while the PCM data is scaled, the PCM format is not changed.
func PCMScale(buf *audio.FloatBuffer, bitDepth int) error {
	if buf == nil || buf.Format == nil {
		return audio.ErrInvalidBuffer
	}
	factor := math.Pow(2, 8*float64(bitDepth/8)-1)
	for i := 0; i < len(buf.Data); i++ {
		buf.Data[i] *= factor
	}

	return nil
}

// PCMScaleF32 converts a buffer with audio content from -1 to 1 (float32) into
// the PCM scale based on the passed bitdepth.
// Note that while the PCM data is scaled, the PCM format is not changed.
func PCMScaleF32(buf *audio.Float32Buffer, bitDepth int) error {
	if buf == nil || buf.Format == nil {
		return audio.ErrInvalidBuffer
	}
	buf.SourceBitDepth = bitDepth
	factor := float32(math.Pow(2, float64(bitDepth)-1)) - 1.0
	for i := 0; i < len(buf.Data); i++ {
		buf.Data[i] *= factor
	}

	return nil
}
