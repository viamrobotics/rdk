package control

import (
	"math"

	"github.com/pkg/errors"
)

type movingAverageFilter struct {
	filterSize  int
	smpCount    int
	x           []float64
	accumulator float64
}

func (f *movingAverageFilter) filterSample(x float64) float64 {
	if f.smpCount < f.filterSize-1 {
		f.smpCount++
		f.accumulator += x
		f.x[f.smpCount] = x
		if f.smpCount == f.filterSize-1 {
			f.accumulator /= float64(f.smpCount)
			return f.accumulator
		}
		return 0.0
	}
	f.accumulator += (x - f.x[0]) / float64(f.smpCount)
	f.x = f.x[1:]
	f.x = append(f.x, x)
	return f.accumulator
}

func (f *movingAverageFilter) Reset() error {
	f.smpCount = 0
	f.x = make([]float64, f.filterSize)
	f.accumulator = 0.0
	return nil
}

func (f *movingAverageFilter) Next(x float64) (float64, bool) {
	y := f.filterSample(x)
	return y, !math.IsNaN(y)
}

type firSinc struct {
	smpFreq    float64
	cutOffFreq float64
	order      int
	coeffs     []float64
	x          []float64
}

func (f *firSinc) calculateCoefficients() error {
	wc := 2.0 * f.cutOffFreq / f.smpFreq
	n1 := (0.5) * (float64(f.order) - 1)
	for i := 0; i < f.order; i++ {
		if float64(i)-n1 == 0 {
			f.coeffs[i] = wc
		} else {
			f.coeffs[i] = math.Sin(wc*math.Pi*(float64(i)-n1)) / (math.Pi * (float64(i) - n1))
		}
	}
	return nil
}

func (f *firSinc) Reset() error {
	f.coeffs = make([]float64, f.order)
	f.x = make([]float64, f.order)
	return f.calculateCoefficients()
}

func (f *firSinc) Next(x float64) (float64, bool) {
	y := x * f.coeffs[0]
	for i := 1; i < f.order; i++ {
		y += f.x[i] * f.coeffs[f.order-i-1]
	}
	f.x = append(f.x, x)
	f.x = f.x[1:]
	return y, true
}

type firWindowedSinc struct {
	smpFreq    float64
	cutOffFreq float64
	kernelSize int
	kernel     []float64
	x          []float64
}

func (f *firWindowedSinc) Reset() error {
	f.x = make([]float64, 0)
	return f.calculateKernel()
}

func (f *firWindowedSinc) Next(x float64) (float64, bool) {
	y := f.filterVal(x)
	if math.IsNaN(y) {
		return 0.0, false
	}
	return y, true
}

func (f *firWindowedSinc) calculateKernel() error {
	wc := 2.0 * f.cutOffFreq / f.smpFreq
	if wc > 1.0 {
		return errors.New("ratio 2 * cutOffFreq/smpFreq cannot be below 1 (Nyquist frequency)")
	}
	f.kernel = make([]float64, f.kernelSize)
	hs := f.kernelSize / 2
	acc := 0.0
	for i := 0; i < f.kernelSize; i++ {
		if i-hs == 0 {
			f.kernel[i] = math.Pi * wc
		} else {
			f.kernel[i] = math.Sin(math.Pi*wc*(float64(i-hs))) / float64(i-hs)
		}
		f.kernel[i] *= (0.54 - 0.46*math.Cos(math.Pi*float64(i/f.kernelSize)))
		acc += f.kernel[i]
	}
	for i := 0; i < f.kernelSize; i++ {
		f.kernel[i] /= acc
	}
	return nil
}

func (f *firWindowedSinc) filterVal(x float64) float64 {
	if len(f.x) < f.kernelSize {
		f.x = append(f.x, x)
		return math.NaN()
	}
	y := 0.0
	for i := 0; i < f.kernelSize; i++ {
		y += f.kernel[i] * f.x[f.kernelSize-i-1]
	}
	f.x = append(f.x, x)
	f.x = f.x[1:]
	return y
}
