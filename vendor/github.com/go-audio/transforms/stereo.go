package transforms

import "github.com/go-audio/audio"

// MonoToStereoF32 converts a mono stream into a stereo one
// by copying the mono signal to both channels in an interleaved
// signal.
func MonoToStereoF32(buf *audio.Float32Buffer) error {
	if buf == nil || buf.Format == nil || buf.Format.NumChannels != 1 {
		return audio.ErrInvalidBuffer
	}
	stereoData := make([]float32, len(buf.Data)*2)
	var j int
	for i := 0; i < len(buf.Data); i++ {
		stereoData[j] = buf.Data[i]
		j++
		stereoData[j] = buf.Data[i]
		j++
	}
	buf.Data = stereoData
	buf.Format.NumChannels = 2
	return nil
}
