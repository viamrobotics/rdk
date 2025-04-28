package transforms

import "github.com/go-audio/audio"

// MonoDownmix converts the buffer to a mono buffer
// by downmixing the channels together.
func MonoDownmix(buf *audio.FloatBuffer) error {
	if buf == nil || buf.Format == nil {
		return audio.ErrInvalidBuffer
	}
	nChans := buf.Format.NumChannels
	if nChans < 2 {
		return nil
	}
	nChansF := float64(nChans)

	frameCount := buf.NumFrames()
	newData := make([]float64, frameCount)
	for i := 0; i < frameCount; i++ {
		newData[i] = 0
		for j := 0; j < nChans; j++ {
			newData[i] += buf.Data[i*nChans+j]
		}
		newData[i] /= nChansF
	}
	buf.Data = newData
	buf.Format.NumChannels = 1

	return nil
}
