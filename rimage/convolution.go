package rimage

import (
	"image"
	"image/color"
	"runtime"
	"sync"
	"sync/atomic"
)

// clamp rounds and clamps float64 value to fit into uint8.
func clampZeroTo255(x float64) uint8 {
	v := int64(x + 0.5)
	if v > 255 {
		return 255
	}
	if v > 0 {
		return uint8(v)
	}
	return 0
}

// Sobel filters are used to approximate the gradient of the image intensity. One filter for each direction.
var (
	//sobelX = [3][3]float64{{-1, 0, 1}, {-2, 0, 2}, {-1, 0, 1}}
	//sobelY = [3][3]float64{{-1, -2, -1}, {0, 0, 0}, {1, 2, 1}}
	maxProcs int64
)


// SetMaxProcs limits the number of concurrent processing goroutines to the given value.
// A value <= 0 clears the limit.
func SetMaxProcs(value int) {
	atomic.StoreInt64(&maxProcs, int64(value))
}

// parallel processes the data in separate goroutines.
func parallel(start, stop int, fn func(<-chan int)) {
	count := stop - start
	if count < 1 {
		return
	}

	procs := runtime.GOMAXPROCS(0)
	limit := int(atomic.LoadInt64(&maxProcs))
	if procs > limit && limit > 0 {
		procs = limit
	}
	if procs > count {
		procs = count
	}

	c := make(chan int, count)
	for i := start; i < stop; i++ {
		c <- i
	}
	close(c)

	var wg sync.WaitGroup
	for i := 0; i < procs; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			fn(c)
		}()
	}
	wg.Wait()
}

// ConvolveOptions are convolution parameters.
type ConvolveOptions struct {
	// If Normalize is true the kernel is normalized before convolution.
	Normalize bool

	// If Abs is true the absolute value of each color channel is taken after convolution.
	Abs bool

	// Bias is added to each color channel value after convolution.
	Bias int
}

// Convolve3x3 convolves the image with the specified 3x3 convolution kernel.
// Default parameters are used if a nil *ConvolveOptions is passed.
func Convolve3x3(img image.Image, kernel [9]float64, stride int, options *ConvolveOptions) *image.NRGBA {
	return convolve(img, kernel[:], stride, options)
}

// Convolve5x5 convolves the image with the specified 5x5 convolution kernel.
// Default parameters are used if a nil *ConvolveOptions is passed.
func Convolve5x5(img image.Image, kernel [25]float64, stride int, options *ConvolveOptions) *image.NRGBA {
	return convolve(img, kernel[:], stride, options)
}

func convolve(img image.Image, kernel []float64, stride int, options *ConvolveOptions) *image.NRGBA {
	src := img
	w := src.Bounds().Max.X
	h := src.Bounds().Max.Y
	dst := image.NewNRGBA(image.Rect(0, 0, w, h))

	if w < 1 || h < 1 {
		return dst
	}

	if options == nil {
		options = &ConvolveOptions{}
	}

	if options.Normalize {
		normalizeKernel(kernel)
	}

	type coef struct {
		x, y int
		k    float64
	}
	var coefs []coef
	var m int

	switch len(kernel) {
	case 9:
		m = 1
	case 25:
		m = 2
	}

	i := 0
	for y := -m; y <= m; y++ {
		for x := -m; x <= m; x++ {
			if kernel[i] != 0 {
				coefs = append(coefs, coef{x: x, y: y, k: kernel[i]})
			}
			i++
		}
	}

	parallel(0, h, func(ys <-chan int) {
		for y := range ys {
			for x := 0; x < w; x++ {
				var r, g, b float64
				for _, c := range coefs {
					ix := x + c.x
					if ix < 0 {
						ix = 0
					} else if ix >= w {
						ix = w - 1
					}

					iy := y + c.y
					if iy < 0 {
						iy = 0
					} else if iy >= h {
						iy = h - 1
					}

					//off := iy*stride + ix*4
					r1, g1, b1, _ := src.At(ix, iy).RGBA()
					// [off : off+3 : off+3]
					r += float64(r1) * c.k
					g += float64(g1) * c.k
					b += float64(b1) * c.k
				}

				if options.Abs {
					if r < 0 {
						r = -r
					}
					if g < 0 {
						g = -g
					}
					if b < 0 {
						b = -b
					}
				}

				if options.Bias != 0 {
					r += float64(options.Bias)
					g += float64(options.Bias)
					b += float64(options.Bias)
				}
				dst.Set(x, y, color.RGBA{clampZeroTo255(r), clampZeroTo255(g), clampZeroTo255(b), 255})

			}
		}
	})

	return dst
}

func normalizeKernel(kernel []float64) {
	var sum, sumpos float64
	for i := range kernel {
		sum += kernel[i]
		if kernel[i] > 0 {
			sumpos += kernel[i]
		}
	}
	if sum != 0 {
		for i := range kernel {
			kernel[i] /= sum
		}
	} else if sumpos != 0 {
		for i := range kernel {
			kernel[i] /= sumpos
		}
	}
}
