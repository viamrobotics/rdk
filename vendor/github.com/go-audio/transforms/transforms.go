package transforms

import (
	"math"

	"github.com/go-audio/audio"
)

// FullWaveRectifier to make all signal positive
// See https://en.wikipedia.org/wiki/Rectifier#Full-wave_rectification
func FullWaveRectifier(buf *audio.FloatBuffer) error {
	if buf == nil {
		return audio.ErrInvalidBuffer
	}
	for i := 0; i < len(buf.Data); i++ {
		buf.Data[i] = math.Abs(buf.Data[i])
	}

	return nil
}

// MonoRMS converts the buffer to mono and apply an RMS treatment.
// rms = sqrt ( (1/n) * (x12 + x22 + … + xn2) )
// multiplying by 1/n effectively assigns equal weights to all the terms, making it a rectangular window.
// Other window equations can be used instead which would favor terms in the middle of the window.
// This results in even greater accuracy of the RMS value since brand new samples (or old ones at
// the end of the window) have less influence over the signal’s power.)
// TODO: use a sine wave at amplitude of 1: rectication + average = 1.4 (1/square root of 2)
func MonoRMS(b *audio.FloatBuffer, windowSize int) error {
	if b == nil {
		return audio.ErrInvalidBuffer
	}
	if len(b.Data) == 0 {
		return nil
	}
	out := []float64{}
	winBuf := make([]float64, windowSize)
	windowSizeF := float64(windowSize)

	processWindow := func(idx int) {
		total := 0.0
		for i := 0; i < len(winBuf); i++ {
			total += winBuf[idx] * winBuf[idx]
		}
		v := math.Sqrt((1.0 / windowSizeF) * total)
		out = append(out, v)
	}

	nbrChans := 1
	if b.Format != nil {
		nbrChans = b.Format.NumChannels
	}

	var windowIDX int
	// process each frame, convert it to mono and them RMS it
	for i := 0; i < len(b.Data); i++ {
		v := b.Data[i]
		if nbrChans > 1 {
			for j := 1; j < nbrChans; j++ {
				i++
				v += b.Data[i]
			}
			v /= float64(nbrChans)
		}
		winBuf[windowIDX] = v
		windowIDX++
		if windowIDX == windowSize || i == (len(b.Data)-1) {
			windowIDX = 0
			processWindow(windowIDX)
		}
	}

	if b.Format != nil {
		b.Format.NumChannels = 1
		b.Format.SampleRate /= windowSize
	}
	b.Data = out
	return nil
}
