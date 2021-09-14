package rimage

import (
	"image"
	"image/color"

	"go.viam.com/core/utils"
	"gonum.org/v1/gonum/mat"
)

// ConvolveGray applies a convolution matrix (kernel) to a grayscale image.
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

// ConvolveRGBA applies a convolution matrix (kernel) to an RGBA image.
// Example of usage:
//
// 		res, err := convolution.ConvolveRGBA(img, kernel, {1, 1}, BorderReflect)
//
// Note: the anchor represents a point inside the area of the kernel. After every step of the convolution the position
// specified by the anchor point gets updated on the result image.
func ConvolveRGBA(img *image.RGBA, kernel *Kernel, anchor image.Point, border BorderPad) (*image.RGBA, error) {
	kernelSize := kernel.Size()
	padded, err := PaddingRGBA(img, kernelSize, anchor, border)
	if err != nil {
		return nil, err
	}
	originalSize := img.Bounds().Size()
	resultImage := image.NewRGBA(img.Bounds())
	utils.ParallelForEachPixel(originalSize, func(x int, y int) {
		sumR, sumG, sumB := 0.0, 0.0, 0.0
		for kx := 0; kx < kernelSize.X; kx++ {
			for ky := 0; ky < kernelSize.Y; ky++ {
				pixel := padded.RGBAAt(x+kx, y+ky)
				sumR += float64(pixel.R) * kernel.At(kx, ky)
				sumG += float64(pixel.G) * kernel.At(kx, ky)
				sumB += float64(pixel.B) * kernel.At(kx, ky)
			}
		}
		sumR = utils.ClampF64(sumR, 0,255)
		sumG = utils.ClampF64(sumG, 0,255)
		sumB = utils.ClampF64(sumB, 0,255)
		rgba := img.RGBAAt(x, y)
		resultImage.Set(x, y, color.RGBA{uint8(sumR), uint8(sumG), uint8(sumB), rgba.A})
	})
	return resultImage, nil
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
