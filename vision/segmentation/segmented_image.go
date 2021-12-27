package segmentation

import (
	"image"
	"image/color"

	"github.com/lucasb-eyer/go-colorful"

	"go.viam.com/rdk/rimage"
)

// SegmentedImage TODO.
type SegmentedImage struct {
	palette []color.Color
	dots    []int //  a value of 0 means no segment, < 0 is transient, > 0 is the segment number
	width   int
	height  int
}

func newSegmentedImage(img *rimage.Image) *SegmentedImage {
	si := &SegmentedImage{
		width:  img.Width(),
		height: img.Height(),
	}
	si.dots = make([]int, si.width*si.height)
	return si
}

// Height TODO.
func (si *SegmentedImage) Height() int {
	return si.height
}

// Width TODO.
func (si *SegmentedImage) Width() int {
	return si.width
}

func (si *SegmentedImage) toK(p image.Point) int {
	return (p.Y * si.width) + p.X
}

func (si *SegmentedImage) fromK(k int) image.Point {
	y := k / si.width
	x := k - (y * si.width)
	return image.Point{x, y}
}

// GetSegment TODO.
func (si *SegmentedImage) GetSegment(p image.Point) int {
	return si.get(p)
}

func (si *SegmentedImage) get(p image.Point) int {
	k := si.toK(p)
	if k < 0 || k >= len(si.dots) {
		return 0
	}
	return si.dots[k]
}

func (si *SegmentedImage) set(p image.Point, val int) {
	k := si.toK(p)
	if k < 0 || k >= len(si.dots) {
		return
	}
	si.dots[k] = val
}

// PixelsInSegmemnt TODO.
func (si *SegmentedImage) PixelsInSegmemnt(segment int) int {
	num := 0
	for _, v := range si.dots {
		if v == segment {
			num++
		}
	}
	return num
}

// ColorModel TODO.
func (si *SegmentedImage) ColorModel() color.Model {
	return color.RGBAModel
}

// Bounds TODO.
func (si *SegmentedImage) Bounds() image.Rectangle {
	return image.Rect(0, 0, si.width, si.height)
}

// At TODO.
func (si *SegmentedImage) At(x, y int) color.Color {
	v := si.get(image.Point{x, y})
	if v <= 0 {
		return color.RGBA{0, 0, 0, 0}
	}
	return si.palette[v-1]
}

func (si *SegmentedImage) createPalette() {
	max := 0
	for _, v := range si.dots {
		if v > max {
			max = v
		}
	}

	if max == 0 {
		// no segments
		return
	}

	palette := colorful.FastWarmPalette(max)

	for _, p := range palette {
		si.palette = append(si.palette, p)
	}
}

// NumInAnyCluster TODO.
func (si *SegmentedImage) NumInAnyCluster() int {
	num := 0
	for _, v := range si.dots {
		if v > 0 {
			num++
		}
	}
	return num
}

func (si *SegmentedImage) clearTransients() {
	for k, v := range si.dots {
		if v < 0 {
			si.dots[k] = 0
		}
	}
}
