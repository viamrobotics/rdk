package rimage

import (
	"errors"
	"image"
	"image/color"

	"gonum.org/v1/gonum/mat"
)

// BorderPad is an enum type for supported padding types.
type BorderPad int

const (
	//nolint:dupword
	// BorderConstant - X X X A B C D E X X X - where X is a black ( color.Gray{0} ) pixel.
	BorderConstant BorderPad = iota
	//nolint:dupword
	// BorderReplicate - A A A A B C D E E E E  - copies the nearest border pixel into padding.
	BorderReplicate
	// BorderReflect - D C B A B C D E D C B - reflects the nearest pixel group around border pixel.
	BorderReflect
)

// Paddings struct holds the padding sizes for each padding.
type Paddings struct {
	// PaddingLeft is the size of the left padding
	PaddingLeft int
	// PaddingRight is the size of the right padding
	PaddingRight int
	// PaddingTop is the size of the top padding
	PaddingTop int
	// PaddingBottom is the size of the bottom padding
	PaddingBottom int
}

func topPaddingReplicate(img image.Image, p Paddings, setPixel func(int, int, color.Color)) {
	originalSize := img.Bounds().Size()
	for x := p.PaddingLeft; x < originalSize.X+p.PaddingLeft; x++ {
		firstPixel := img.At(x-p.PaddingLeft, p.PaddingTop)
		for y := 0; y < p.PaddingTop; y++ {
			setPixel(x, y, firstPixel)
		}
	}
}

func bottomPaddingReplicate(img image.Image, p Paddings, setPixel func(int, int, color.Color)) {
	originalSize := img.Bounds().Size()
	for x := p.PaddingLeft; x < originalSize.X+p.PaddingLeft; x++ {
		lastPixel := img.At(x-p.PaddingLeft, originalSize.Y-1)
		for y := p.PaddingTop + originalSize.Y; y < originalSize.Y+p.PaddingTop+p.PaddingBottom; y++ {
			setPixel(x, y, lastPixel)
		}
	}
}

func leftPaddingReplicate(img, padded image.Image, p Paddings, setPixel func(int, int, color.Color)) {
	originalSize := img.Bounds().Size()
	for y := 0; y < originalSize.Y+p.PaddingBottom+p.PaddingTop; y++ {
		firstPixel := padded.At(p.PaddingLeft, y)
		for x := 0; x < p.PaddingLeft; x++ {
			setPixel(x, y, firstPixel)
		}
	}
}

func rightPaddingReplicate(img, padded image.Image, p Paddings, setPixel func(int, int, color.Color)) {
	originalSize := img.Bounds().Size()
	for y := 0; y < originalSize.Y+p.PaddingBottom+p.PaddingTop; y++ {
		lastPixel := padded.At(originalSize.X+p.PaddingLeft-1, y)
		for x := originalSize.X + p.PaddingLeft; x < originalSize.X+p.PaddingLeft+p.PaddingRight; x++ {
			setPixel(x, y, lastPixel)
		}
	}
}

func topPaddingReflect(img image.Image, p Paddings, setPixel func(int, int, color.Color)) {
	originalSize := img.Bounds().Size()
	for x := p.PaddingLeft; x < originalSize.X+p.PaddingLeft; x++ {
		for y := 0; y < p.PaddingTop; y++ {
			pixel := img.At(x-p.PaddingLeft, p.PaddingTop-y)
			setPixel(x, y, pixel)
		}
	}
}

func bottomPaddingReflect(img image.Image, p Paddings, setPixel func(int, int, color.Color)) {
	originalSize := img.Bounds().Size()
	for x := p.PaddingLeft; x < originalSize.X+p.PaddingLeft; x++ {
		for y := p.PaddingTop + originalSize.Y; y < originalSize.Y+p.PaddingTop+p.PaddingBottom; y++ {
			pixel := img.At(x-p.PaddingLeft, originalSize.Y-(y-p.PaddingTop-originalSize.Y)-2)
			setPixel(x, y, pixel)
		}
	}
}

func leftPaddingReflect(img, padded image.Image, p Paddings, setPixel func(int, int, color.Color)) {
	originalSize := img.Bounds().Size()
	for y := 0; y < originalSize.Y+p.PaddingBottom+p.PaddingTop; y++ {
		for x := 0; x < p.PaddingLeft; x++ {
			pixel := padded.At(2*p.PaddingLeft-x, y)
			setPixel(x, y, pixel)
		}
	}
}

func rightPaddingReflect(img, padded image.Image, p Paddings, setPixel func(int, int, color.Color)) {
	originalSize := img.Bounds().Size()
	for y := 0; y < originalSize.Y+p.PaddingBottom+p.PaddingTop; y++ {
		for x := originalSize.X + p.PaddingLeft; x < originalSize.X+p.PaddingLeft+p.PaddingRight; x++ {
			pixel := padded.At(originalSize.X+p.PaddingLeft-(x-originalSize.X-p.PaddingLeft)-2, y)
			setPixel(x, y, pixel)
		}
	}
}

// PaddingFloat64 pads a *mat.Dense - padding mode = reflect.
func PaddingFloat64(img *mat.Dense, kernelSize, anchor image.Point, border BorderPad) (*mat.Dense, error) {
	h, w := img.Dims()
	originalSize := image.Point{w, h}
	p, err := computePaddingSizes(kernelSize, anchor)
	if err != nil {
		return nil, err
	}
	rect := getImageRectangleFromPaddingSizes(p, originalSize)
	newW, newH := rect.Max.X, rect.Max.Y
	padded := mat.NewDense(newH, newW, nil)

	for x := p.PaddingLeft; x < originalSize.X+p.PaddingLeft; x++ {
		for y := p.PaddingTop; y < originalSize.Y+p.PaddingTop; y++ {
			padded.Set(y, x, img.At(y-p.PaddingTop, x-p.PaddingLeft))
		}
	}
	return padded, nil
}

// PaddingGray appends padding to a given grayscale image. The size of the padding is calculated from the kernel size
// and the anchor point. Supported Border types are: BorderConstant, BorderReplicate, BorderReflect.
// Example of usage:
//
//	res, err := padding.PaddingGray(img, {5, 5}, {1, 1}, BorderReflect)
//
// Note: this will add a 1px padding for the top and left borders of the image and a 3px padding fot the bottom and
// right borders of the image.
func PaddingGray(img *image.Gray, kernelSize, anchor image.Point, border BorderPad) (*image.Gray, error) {
	originalSize := img.Bounds().Size()
	p, err := computePaddingSizes(kernelSize, anchor)
	if err != nil {
		return nil, err
	}
	rect := getImageRectangleFromPaddingSizes(p, originalSize)
	padded := image.NewGray(rect)

	for x := p.PaddingLeft; x < originalSize.X+p.PaddingLeft; x++ {
		for y := p.PaddingTop; y < originalSize.Y+p.PaddingTop; y++ {
			padded.Set(x, y, img.GrayAt(x-p.PaddingLeft, y-p.PaddingTop))
		}
	}

	switch border {
	case BorderConstant:
		// do nothing
	case BorderReplicate:
		topPaddingReplicate(img, p, func(x, y int, pixel color.Color) {
			padded.Set(x, y, pixel)
		})
		bottomPaddingReplicate(img, p, func(x, y int, pixel color.Color) {
			padded.Set(x, y, pixel)
		})
		leftPaddingReplicate(img, padded, p, func(x, y int, pixel color.Color) {
			padded.Set(x, y, pixel)
		})
		rightPaddingReplicate(img, padded, p, func(x, y int, pixel color.Color) {
			padded.Set(x, y, pixel)
		})
	case BorderReflect:
		topPaddingReflect(img, p, func(x, y int, pixel color.Color) {
			padded.Set(x, y, pixel)
		})
		bottomPaddingReflect(img, p, func(x, y int, pixel color.Color) {
			padded.Set(x, y, pixel)
		})
		leftPaddingReflect(img, padded, p, func(x, y int, pixel color.Color) {
			padded.Set(x, y, pixel)
		})
		rightPaddingReflect(img, padded, p, func(x, y int, pixel color.Color) {
			padded.Set(x, y, pixel)
		})
	default:
		return nil, errors.New("unknown Border type")
	}
	return padded, nil
}

// helper functions.
func computePaddingSizes(kernelSize, anchor image.Point) (Paddings, error) {
	var p Paddings
	if kernelSize.X < 0 || kernelSize.Y < 0 {
		return p, errors.New("kernel size is negative")
	}
	if anchor.X < 0 || anchor.Y < 0 {
		return p, errors.New("anchor value is negative")
	}
	if anchor.X > kernelSize.X || anchor.Y > kernelSize.Y {
		return p, errors.New("anchor value outside of the kernel")
	}

	p = Paddings{
		PaddingLeft: anchor.X, PaddingRight: kernelSize.X - anchor.X - 1,
		PaddingTop: anchor.Y, PaddingBottom: kernelSize.Y - anchor.Y - 1,
	}

	return p, nil
}

func getImageRectangleFromPaddingSizes(p Paddings, imgSize image.Point) image.Rectangle {
	x := p.PaddingLeft + p.PaddingRight + imgSize.X
	y := p.PaddingTop + p.PaddingBottom + imgSize.Y
	return image.Rect(0, 0, x, y)
}
