package control

import (
	"math"

	"github.com/pkg/errors"
)

// All IIR filters are implemented as y[n] = a_0*x[n] + a_1*x[n-1] + ... + a_n*x[n] - ( b_1*y[n-1] + b_2*y[n-2] + ... + b_n*y[n])

type iirFilter struct {
	smpFreq        float64   // smpFreq sampling frequency of the signal
	n              int       // n order of the filt
	cutOffFreq     float64   // cutOffFreq cut off frequency from [0,pi]
	ripple         float64   // Ripple factor for Chebyshev Type I
	fltType        string    // type [Lowpass, Highpass]
	x              []float64 // x hold n previous values
	y              []float64 // y hold n previous values
	aCoeffs        []float64 // aCoeffs a coefficients
	bCoeffs        []float64 // bCoeffs b coefficients
	normalizedGain float64   // normalizedGain will make the filter have a gain of one at DC
}

func calculateBiquadCoefficient(fc float64, p, n int, rp float64, hp bool) ([]float64, []float64) {
	a := make([]float64, 3)
	b := make([]float64, 3)
	realP := -math.Cos(math.Pi/float64(2*n) + float64(p-1)*math.Pi/float64(n))
	imagP := math.Sin(math.Pi/float64(n*2) + float64(p-1)*math.Pi/float64(n))

	if rp > 0 {
		scale := math.Sqrt(math.Pow(100.0/(100.0-rp), 2) - 1)
		vx := (1.0 / float64(n)) * math.Log((1.0/scale)+math.Sqrt((math.Pow(1.0/scale, 2))+1))
		kx := (1.0 / float64(n)) * math.Log((1.0/scale)+math.Sqrt((math.Pow(1.0/scale, 2))-1))
		kx = (math.Exp(kx) + math.Exp(-kx)) * 0.5
		realP = realP * ((math.Exp(vx) - math.Exp(-vx)) * 0.5) / kx
		imagP = imagP * ((math.Exp(vx) + math.Exp(-vx)) * 0.5) / kx
	}
	T := 2 * math.Tan(0.5)
	Wc := math.Pi * fc
	squaredMod := math.Pow(realP, 2) + math.Pow(imagP, 2)
	d := 4 - 4*realP*T + squaredMod*math.Pow(T, 2)
	x0 := (math.Pow(T, 2)) / d
	x1 := 2 * (math.Pow(T, 2)) / d
	x2 := (math.Pow(T, 2)) / d
	y1 := (8 - 2*squaredMod*math.Pow(T, 2)) / d
	y2 := (-4 - 4*realP*T - squaredMod*math.Pow(T, 2)) / d
	var k float64
	if !hp {
		k = math.Sin(0.5-Wc/2) / math.Sin(0.5+Wc/2)
	} else {
		k = -math.Cos(0.5+Wc/2) / math.Cos(-0.5+Wc/2)
	}
	d = 1 + y1*k - y2*math.Pow(k, 2)

	a[0] = (x0 - x1*k + x2*math.Pow(k, 2)) / d
	a[1] = (-2*x0*k + x1 + x1*math.Pow(k, 2) - 2*x2*k) / d
	a[2] = (x0*math.Pow(k, 2) - x1*k + x2) / d
	b[0] = 1
	b[1] = (2*k + y1 + y1*math.Pow(k, 2) - 2*y2*k) / d
	b[2] = (-math.Pow(k, 2) - y1*k + y2) / d
	if hp {
		a[1] *= -1.0
		b[1] *= -1.0
	}
	return a, b
}

// calculateABCoeffs calculate the a,b coefficient for the recursive filter function.
// To simplify the algebra we use a cascade of biquad filters for order > 2
// hence we can only build even order filters.
func (f *iirFilter) calculateABCoeffs() error {
	fc := 2.0 * f.cutOffFreq / f.smpFreq

	if fc > 1 {
		return errors.New("ratio 2 * cutOffFreq/smpFreq cannot be below 1 (Nyquist frequency)")
	}
	if f.n%2 != 0 {
		return errors.New("order of the filter must be an even number")
	}
	np := f.n / 2
	f.aCoeffs = make([]float64, f.n+3)
	f.bCoeffs = make([]float64, f.n+3)
	taCoeffs := make([]float64, f.n+3)
	tbCoeffs := make([]float64, f.n+3)

	f.aCoeffs[2] = 1
	f.bCoeffs[2] = 1

	for i := 1; i < np+1; i++ {
		for j := 0; j < len(f.bCoeffs); j++ {
			taCoeffs[j] = f.aCoeffs[j]
			tbCoeffs[j] = f.bCoeffs[j]
		}
		var a, b []float64
		if f.fltType == "highpass" {
			a, b = calculateBiquadCoefficient(fc, i, f.n, f.ripple, true)
		} else {
			a, b = calculateBiquadCoefficient(fc, i, f.n, f.ripple, false)
		}
		for j := 2; j < len(f.bCoeffs); j++ {
			f.aCoeffs[j] = a[0]*taCoeffs[j] + a[1]*taCoeffs[j-1] + a[2]*taCoeffs[j-2]
			f.bCoeffs[j] = b[0]*tbCoeffs[j] - b[1]*tbCoeffs[j-1] - b[2]*tbCoeffs[j-2]
		}
	}
	f.bCoeffs[2] = 0
	for i := 0; i < len(f.aCoeffs)-2; i++ {
		f.aCoeffs[i] = f.aCoeffs[i+2]
		f.bCoeffs[i] = f.bCoeffs[i+2]
	}
	Ga := 0.0
	Gb := 0.0
	f.aCoeffs = f.aCoeffs[:len(f.aCoeffs)-2]
	f.bCoeffs = f.bCoeffs[:len(f.bCoeffs)-2]
	for i := 0; i < len(f.aCoeffs); i++ {
		one := 1.0
		if f.fltType == "highpass" {
			one = math.Pow(-1.0, float64(i))
		}
		Ga += f.aCoeffs[i] * one
		Gb += -f.bCoeffs[i] * one
	}
	G := Ga / (float64(1.0) - Gb)
	f.normalizedGain = G
	return nil
}

func (f *iirFilter) Next(x float64) (float64, bool) {
	y := x * f.aCoeffs[0] / f.normalizedGain
	for n := 1; n < f.n+1; n++ {
		y += (f.aCoeffs[n]/f.normalizedGain)*f.x[f.n-n] - f.bCoeffs[n]*f.y[f.n-n]
	}
	f.x = append(f.x, x)
	f.x = f.x[1:]
	f.y = append(f.y, y)
	f.y = f.y[1:]
	return y, true
}

func (f *iirFilter) Reset() error {
	f.x = make([]float64, f.n)
	f.y = make([]float64, f.n)
	err := f.calculateABCoeffs()
	return err
}

func design(fp, fs, gp, gs, smpFreq float64) (*iirFilter, error) {
	wp := 2.0 * fp / smpFreq
	ws := 2.0 * fs / smpFreq
	if wp > 1.0 {
		return nil, errors.New("passband frequency should be between [0,0.5*fs]")
	}
	if ws > 1.0 {
		return nil, errors.New("stopband frequency should be between [0,0.5*fs]")
	}
	wp = math.Tan(math.Pi * wp / 2.0)
	ws = math.Tan(math.Pi * ws / 2.0)

	// TODO this is just for lowpass
	n := int(math.Ceil(math.Log10((math.Pow(10.0, gs/10)-1)/(math.Pow(10.0, gp/10)-1)) / (2 * math.Log10(ws/wp))))
	if n%2 != 0 {
		n++
	}
	if n == 0 {
		return nil, errors.New("filter order is 0")
	}
	wc := wp / math.Pow((math.Pow(10.0, gp/10.0)-1), 1.0/(2.0*float64(n)))
	fc := (1.0 / math.Pi) * math.Atan(wc) * smpFreq
	return &iirFilter{smpFreq: smpFreq, cutOffFreq: fc, n: n, ripple: 0.0, fltType: "lowpass"}, nil
}
