//go:build !notc

package rimage

import (
	"bytes"
	"context"
	"image"
	"image/color"

	"github.com/pkg/errors"
	"go.viam.com/utils"
)

// Overlay overlays an rgb image over a depth map.
func Overlay(i *Image, dm *DepthMap) *image.NRGBA {
	const minAlpha = 32.0

	min, max := dm.MinMax()

	img := image.NewNRGBA(i.Bounds())
	for x := 0; x < i.Width(); x++ {
		for y := 0; y < i.Height(); y++ {
			c := i.GetXY(x, y)

			a := uint8(0)

			d := dm.GetDepth(x, y)
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

// imageWithDepth is an image of color that has depth associated with it.
// It may or may not be aligned. It fulfills the image.Image interface.
type imageWithDepth struct {
	Color   *Image
	Depth   *DepthMap
	aligned bool
}

// makeImageWithDepth returns a new image with depth from the given color and depth arguments.
// aligned is true if the two channels are aligned, false if not.
func makeImageWithDepth(img *Image, dm *DepthMap, aligned bool) *imageWithDepth {
	return &imageWithDepth{img, dm, aligned}
}

// Bounds returns the bounds.
func (i *imageWithDepth) Bounds() image.Rectangle {
	return i.Color.Bounds()
}

// ColorModel returns the color model of the color image.
func (i *imageWithDepth) ColorModel() color.Model {
	return i.Color.ColorModel()
}

// Clone makes a copy of the image with depth.
func (i *imageWithDepth) Clone() *imageWithDepth {
	if i.Color == nil {
		return &imageWithDepth{nil, nil, i.aligned}
	}
	if i.Depth == nil {
		return &imageWithDepth{i.Color.Clone(), nil, i.aligned}
	}
	return &imageWithDepth{i.Color.Clone(), i.Depth.Clone(), i.aligned}
}

// At returns the color at the given point.
// TODO(erh): alpha encode with depth.
func (i *imageWithDepth) At(x, y int) color.Color {
	return i.Color.At(x, y)
}

// IsAligned returns if the image and depth are aligned.
func (i *imageWithDepth) IsAligned() bool {
	return i.aligned
}

// Width returns the horizontal width of the image.
func (i *imageWithDepth) Width() int {
	return i.Color.Width()
}

// Height returns the vertical height of the image.
func (i *imageWithDepth) Height() int {
	return i.Color.Height()
}

// SubImage returns the crop of the image defined by the given rectangle.
func (i *imageWithDepth) SubImage(r image.Rectangle) *imageWithDepth {
	if r.Empty() {
		return &imageWithDepth{}
	}
	return &imageWithDepth{i.Color.SubImage(r), i.Depth.SubImage(r), i.aligned}
}

// Rotate rotates the color and depth about the origin by the given angle clockwise.
func (i *imageWithDepth) Rotate(amount int) *imageWithDepth {
	return &imageWithDepth{i.Color.Rotate(amount), i.Depth.Rotate(amount), i.aligned}
}

// CropToDepthData TODO.
func (i *imageWithDepth) CropToDepthData() (*imageWithDepth, error) {
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
		return nil, errors.Errorf("invalid depth data: %v %v", minY, maxY)
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

	col, dep := WarpColorDepth(i.Color, i.Depth,
		[]image.Point{{minX, minY}, {maxX, minY}, {maxX, maxY}, {minX, maxY}},
		[]image.Point{{0, 0}, {width, 0}, {width, height}, {0, height}},
		image.Point{width, height},
	)
	return &imageWithDepth{col, dep, i.aligned}, nil
}

// Overlay TODO.
func (i *imageWithDepth) Overlay() *image.NRGBA {
	return Overlay(i.Color, i.Depth)
}

// newImageWithDepth returns a new image from the given color image and depth data files.
func newImageWithDepth(ctx context.Context, colorFN, depthFN string, isAligned bool) (*imageWithDepth, error) {
	img, err := NewImageFromFile(colorFN)
	if err != nil {
		return nil, errors.Wrapf(err, "cannot read color file (%s)", colorFN)
	}

	dm, err := NewDepthMapFromFile(ctx, depthFN)
	if err != nil {
		return nil, errors.Wrapf(err, "cannot read depth file (%s)", depthFN)
	}

	if isAligned {
		if img.Width() != dm.Width() || img.Height() != dm.Height() {
			return nil, errors.Errorf("color and depth size doesn't match %d,%d vs %d,%d",
				img.Width(), img.Height(), dm.Width(), dm.Height())
		}
	}

	return &imageWithDepth{img, dm, isAligned}, nil
}

// convertToImageWithDepth attempts to convert a go image into an image
// with depth, if it contains any.
func convertToImageWithDepth(img image.Image) *imageWithDepth {
	switch x := img.(type) {
	case *imageWithDepth:
		return x
	case *DepthMap:
		return &imageWithDepth{ConvertImage(x), x, true}
	case *Image:
		return &imageWithDepth{x, nil, false}
	default:
		return &imageWithDepth{ConvertImage(img), nil, false}
	}
}

// cloneToImageWithDepth attempts to clone a go image into an image
// with depth, if it contains any.
func cloneToImageWithDepth(img image.Image) *imageWithDepth {
	switch x := img.(type) {
	case *imageWithDepth:
		return x.Clone()
	case *Image:
		return &imageWithDepth{x.Clone(), nil, false}
	default:
		return &imageWithDepth{CloneImage(img), nil, false}
	}
}

// RawBytesWrite writes out the internal representation of the color
// and depth into the given buffer.
func (i *imageWithDepth) RawBytesWrite(buf *bytes.Buffer) error {
	if i.Color == nil || i.Depth == nil {
		return errors.New("for raw bytes need depth and color info")
	}

	if i.Color.Width() != i.Depth.Width() {
		return errors.New("widths don't match")
	}

	if i.Color.Height() != i.Depth.Height() {
		return errors.New("heights don't match")
	}

	buf.Write(utils.RawBytesFromSlice(i.Depth.data))
	buf.Write(utils.RawBytesFromSlice(i.Color.data))
	if i.IsAligned() {
		buf.WriteByte(0x1)
	} else {
		buf.WriteByte(0x0)
	}

	return nil
}

// WarpColorDepth adapts the image to a new size.
func WarpColorDepth(col *Image, dm *DepthMap, src, dst []image.Point, newSize image.Point) (*Image, *DepthMap) {
	m2 := GetPerspectiveTransform(src, dst)

	img := WarpImage(col, m2, newSize)

	var warpedDepth *DepthMap
	if dm != nil && dm.Width() > 0 {
		warpedDepth = dm.Warp(m2, newSize)
	}

	return ConvertImage(img), warpedDepth
}
