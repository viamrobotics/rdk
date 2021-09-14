package rimage

import (
	"image"
	"image/color"

	"gonum.org/v1/gonum/mat"

	"go.viam.com/core/utils"
)

// GetSobelX returns the Kernel corresponding to the Sobel kernel in the x direction
func GetSobelX() Kernel {

	return Kernel{[][]float64{
		{-1, 0, 1},
		{-2, 0, 2},
		{-1, 0, 1},
	},
		3,
		3,
	}
}

// GetSobelY returns the Kernel corresponding to the Sobel kernel in the y direction
func GetSobelY() Kernel {

	return Kernel{[][]float64{
		{-1, -2, -1},
		{0, 0, 0},
		{1, 2, 1},
	},
		3, //bias
		3, //factor
	}
}

// ConvolveGray applies a convolution matrix (Kernel) to a grayscale image.
// Example of usage:
//
// 		res, err := convolution.ConvolveGray(img, kernel, {1, 1}, BorderReflect)
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
	utils.ParallelForEachPixel(originalSize, func(x int, y int) {
		sum := float64(0)
		for ky := 0; ky < kernelSize.Y; ky++ {
			for kx := 0; kx < kernelSize.X; kx++ {
				pixel := padded.GrayAt(x+kx, y+ky)
				kE := kernel.At(kx, ky)
				sum += float64(pixel.Y) * kE
			}
		}
		sum = utils.ClampF64(sum, 0, 255)
		resultImage.Set(x, y, color.Gray{uint8(sum)})
	})
	return resultImage, nil
}

// ConvolveGrayFloat64 implements a gray float64 image convolution with the Kernel filter
// There is no clamping in this case
func ConvolveGrayFloat64(m *mat.Dense, filter *Kernel) (*mat.Dense, error) {
	h, w := m.Dims()
	result := mat.NewDense(h, w, nil)
	kernelSize := filter.Size()
	padded, err := PaddingFloat64(m, kernelSize, image.Point{1, 1}, 0)
	if err != nil {
		return nil, err
	}

	utils.ParallelForEachPixel(image.Point{w, h}, func(x int, y int) {
		sum := float64(0)
		for ky := 0; ky < kernelSize.Y; ky++ {
			for kx := 0; kx < kernelSize.X; kx++ {
				pixel := padded.At(y+ky, x+kx)
				kE := filter.At(kx, ky)
				sum += pixel * kE
			}
		}
		result.Set(y, x, sum)
	})
	return result, nil
}

func ConvolveGrayFloat64(m *mat.Dense, filter *Kernel) (*mat.Dense, error) {
	h, w := m.Dims()
	result := mat.NewDense(h, w, nil)
	kernelSize := filter.Size()
	padded, err := PaddingFloat64(m, kernelSize, image.Point{1, 1}, 0)
	if err != nil {
		return nil, err
	}

	utils.ParallelForEachPixel(image.Point{w, h}, func(x int, y int) {
		sum := float64(0)
		for ky := 0; ky < kernelSize.Y; ky++ {
			for kx := 0; kx < kernelSize.X; kx++ {
				pixel := padded.At(y+ky, x+kx)
				kE := filter.At(kx, ky)
				sum += pixel * kE
			}
		}
		//sum = utils.ClampF64(sum, utils.MinUint8, float64(utils.MaxUint8))
		result.Set(y, x, sum)
	})
	return result, nil
}
