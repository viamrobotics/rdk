package rimage

import (
	"fmt"
	"image"
	"image/color"
)

type ImageWithDepth struct {
	Color   *Image
	Depth   *DepthMap
	aligned bool
	aligner DepthColorAligner
}

func MakeImageWithDepth(img *Image, dm *DepthMap, aligned bool, aligner DepthColorAligner) *ImageWithDepth {
	return &ImageWithDepth{img, dm, aligned, aligner}
}

func (i *ImageWithDepth) Bounds() image.Rectangle {
	return i.Color.Bounds()
}

func (i *ImageWithDepth) ColorModel() color.Model {
	return i.Color.ColorModel()
}

func (i *ImageWithDepth) At(x, y int) color.Color {
	return i.Color.At(x, y) // TODO(erh): alpha encode with depth
}

func (i *ImageWithDepth) Width() int {
	return i.Color.Width()
}

func (i *ImageWithDepth) Height() int {
	return i.Color.Height()
}

func (i *ImageWithDepth) Rotate(amount int) *ImageWithDepth {
	return &ImageWithDepth{i.Color.Rotate(amount), i.Depth.Rotate(amount), i.aligned, i.aligner}
}

func (i *ImageWithDepth) Warp(src, dst []image.Point, newSize image.Point) *ImageWithDepth {
	m2 := GetPerspectiveTransform(src, dst)

	img := WarpImage(i.Color, m2, newSize)

	var warpedDepth *DepthMap
	if i.Depth != nil && i.Depth.Width() > 0 {
		dm2 := i.Depth.Warp(m2, newSize)
		warpedDepth = &dm2
	}

	return &ImageWithDepth{ConvertImage(img), warpedDepth, i.aligned, i.aligner}
}

func (i *ImageWithDepth) CropToDepthData() (*ImageWithDepth, error) {
	var minY, minX, maxY, maxX int

	for minY = 0; minY < i.Height(); minY++ {
		found := false
		for x := 0; x < i.Width(); x++ {
			if i.Depth.GetDepth(x, minY) > 0 {
				found = true
				break
			}
		}
		if found {
			break
		}
	}

	for maxY = i.Height() - 1; maxY >= 0; maxY-- {
		found := false
		for x := 0; x < i.Width(); x++ {
			if i.Depth.GetDepth(x, maxY) > 0 {
				found = true
				break
			}
		}
		if found {
			break
		}
	}

	if maxY <= minY {
		return nil, fmt.Errorf("invalid depth data: %v %v", minY, maxY)
	}

	for minX = 0; minX < i.Width(); minX++ {
		found := false
		for y := minY; y < maxY; y++ {
			if i.Depth.GetDepth(minX, y) > 0 {
				found = true
				break
			}
		}
		if found {
			break
		}
	}

	for maxX = i.Width() - 1; minX >= 0; maxX-- {
		found := false
		for y := minY; y < maxY; y++ {
			if i.Depth.GetDepth(maxX, y) > 0 {
				found = true
				break
			}
		}
		if found {
			break
		}
	}

	height := maxY - minY
	width := maxX - minX

	return i.Warp(
		[]image.Point{{minX, minY}, {maxX, minY}, {maxX, maxY}, {minX, maxY}},
		[]image.Point{{0, 0}, {width, 0}, {width, height}, {0, height}},
		image.Point{width, height},
	), nil
}

func (i *ImageWithDepth) Overlay() *image.NRGBA {
	const minAlpha = 32.0

	min, max := i.Depth.MinMax()

	img := image.NewNRGBA(i.Bounds())
	for x := 0; x < i.Width(); x++ {
		for y := 0; y < i.Height(); y++ {
			c := i.Color.GetXY(x, y)

			a := uint8(0)

			d := i.Depth.GetDepth(x, y)
			if d > 0 {
				diff := d - min
				scale := 1.0 - (float64(diff) / float64(max-min))
				a = uint8(minAlpha + ((255.0 - minAlpha) * scale))
			}

			r, g, b := c.RGB255()
			img.SetNRGBA(x, y, color.NRGBA{r, g, b, a})

		}
	}
	return img
}

func (i *ImageWithDepth) WriteTo(fn string) error {
	return BothWriteToFile(i, fn)
}

func NewImageWithDepthFromImages(colorFN, depthFN string, isAligned bool) (*ImageWithDepth, error) {
	img, err := NewImageFromFile(colorFN)
	if err != nil {
		return nil, err
	}

	dm, err := NewDepthMapFromImageFile(depthFN)
	if err != nil {
		return nil, err
	}

	return &ImageWithDepth{img, dm, isAligned, nil}, nil
}

func NewImageWithDepth(colorFN, depthFN string, isAligned bool) (*ImageWithDepth, error) {
	img, err := NewImageFromFile(colorFN)
	if err != nil {
		return nil, err
	}

	dm, err := ParseDepthMap(depthFN)
	if err != nil {
		return nil, err
	}

	if img.Width() != dm.Width() || img.Height() != dm.Height() {
		return nil, fmt.Errorf("color and depth size doesn't match %d,%d vs %d,%d",
			img.Width(), img.Height(), dm.Width(), dm.Height())
	}

	return &ImageWithDepth{img, dm, isAligned, nil}, nil
}

func imageToDepthMap(img image.Image) *DepthMap {
	bounds := img.Bounds()

	width, height := bounds.Dx(), bounds.Dy()
	dm := NewEmptyDepthMap(width, height)

	grayImg := img.(*image.Gray16)
	for x := 0; x < width; x++ {
		for y := 0; y < height; y++ {
			i := grayImg.PixOffset(x, y)
			z := uint16(grayImg.Pix[i+0])<<8 | uint16(grayImg.Pix[i+1])
			dm.Set(x, y, Depth(z))
		}
	}

	return &dm
}

func ConvertToImageWithDepth(img image.Image) *ImageWithDepth {
	switch x := img.(type) {
	case *ImageWithDepth:
		return x
	case *Image:
		return &ImageWithDepth{x, nil, false, nil}
	default:
		return &ImageWithDepth{ConvertImage(img), nil, false, nil}
	}
}
