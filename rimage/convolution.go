package rimage

import (
	"image"
	"image/color"
	"math"

	"gonum.org/v1/gonum/mat"

	"go.viam.com/rdk/utils"
)

// GetSobelX returns the Kernel corresponding to the Sobel kernel in the x direction.
func GetSobelX() Kernel {
	return Kernel{
		[][]float64{
			{-1, 0, 1},
			{-2, 0, 2},
			{-1, 0, 1},
		},
		3,
		3,
	}
}

// GetSobelY returns the Kernel corresponding to the Sobel kernel in the y direction.
func GetSobelY() Kernel {
	return Kernel{
		[][]float64{
			{-1, -2, -1},
			{0, 0, 0},
			{1, 2, 1},
		},
		3,
		3,
	}
}

// GetBlur3 returns the Kernel corresponding to a mean averaging kernel.
func GetBlur3() Kernel {
	return Kernel{
		[][]float64{
			{1, 1, 1},
			{1, 1, 1},
			{1, 1, 1},
		},
		3,
		3,
	}
}

// GetGaussian3 returns the Kernel corresponding to 3x3 Gaussian blurring kernel.
func GetGaussian3() Kernel {
	return Kernel{
		[][]float64{
			{1, 2, 1},
			{2, 4, 2},
			{1, 2, 1},
		},
		3,
		3,
	}
}

// GetGaussian5 returns the Kernel corresponding to 5x5 Gaussian blurring kernel.
func GetGaussian5() Kernel {
	return Kernel{
		[][]float64{
			{1, 4, 7, 4, 1},
			{4, 16, 26, 16, 4},
			{7, 26, 41, 26, 7},
			{4, 16, 26, 16, 4},
			{1, 4, 7, 4, 1},
		},
		5,
		5,
	}
}

// ConvolveGray applies a convolution matrix (Kernel) to a grayscale image.
// Example of usage:
//
//	res, err := convolution.ConvolveGray(img, kernel, {1, 1}, BorderReflect)
//
// Note: the anchor represents a point inside the area of the kernel. After every step of the convolution the position
// specified by the anchor point gets updated on the result image.
func ConvolveGray(img *image.Gray, kernel *Kernel, anchor image.Point, border BorderPad) (*image.Gray, error) {
	kernelSize := kernel.Size()
	padded, err := PaddingGray(img, kernelSize, anchor, border)
	if err != nil {
		return nil, err
	}
	originalSize := img.Bounds().Size()
	resultImage := image.NewGray(img.Bounds())
	utils.ParallelForEachPixel(originalSize, func(x, y int) {
		sum := float64(0)
		for ky := 0; ky < kernelSize.Y; ky++ {
			for kx := 0; kx < kernelSize.X; kx++ {
				pixel := padded.GrayAt(x+kx, y+ky)
				kE := kernel.At(kx, ky)
				sum += float64(pixel.Y) * kE
			}
		}
		sum = utils.Clamp(sum, 0, 255)
		resultImage.Set(x, y, color.Gray{uint8(sum)})
	})
	return resultImage, nil
}

// ConvolveGrayFloat64 implements a gray float64 image convolution with the Kernel filter
// There is no clamping in this case.
func ConvolveGrayFloat64(m *mat.Dense, filter *Kernel) (*mat.Dense, error) {
	h, w := m.Dims()
	result := mat.NewDense(h, w, nil)
	kernelSize := filter.Size()
	padded, err := PaddingFloat64(m, kernelSize, image.Point{1, 1}, 0)
	if err != nil {
		return nil, err
	}

	utils.ParallelForEachPixel(image.Point{w, h}, func(x, y int) {
		sum := float64(0)
		for ky := 0; ky < kernelSize.Y; ky++ {
			for kx := 0; kx < kernelSize.X; kx++ {
				pixel := padded.At(y+ky, x+kx)
				kE := filter.At(ky, kx)
				sum += pixel * kE
			}
		}
		sum = math.Floor(sum)
		result.Set(y, x, sum)
	})
	return result, nil
}
