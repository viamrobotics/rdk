package rimage

import (
	"bytes"
	"image"
	"image/color"

	"github.com/golang/geo/r3"
	"github.com/pkg/errors"
	"go.viam.com/utils"

	rdkutils "go.viam.com/rdk/utils"
)

// ImageWithDepth is an image of color that has depth associated
// with it. It may or may not be aligned.
type ImageWithDepth struct {
	Color   *Image
	Depth   *DepthMap
	aligned bool
	camera  Projector
}

// MakeImageWithDepth returns a new image with depth from the given color and depth arguments. It
// is also associated with a camera that captured the color and depth.
func MakeImageWithDepth(img *Image, dm *DepthMap, aligned bool, camera Projector) *ImageWithDepth {
	return &ImageWithDepth{img, dm, aligned, camera}
}

// Bounds returns the bounds.
func (i *ImageWithDepth) Bounds() image.Rectangle {
	return i.Color.Bounds()
}

// ColorModel returns the color model of the color image.
func (i *ImageWithDepth) ColorModel() color.Model {
	return i.Color.ColorModel()
}

// At returns the color at the given point.
// TODO(erh): alpha encode with depth.
func (i *ImageWithDepth) At(x, y int) color.Color {
	return i.Color.At(x, y)
}

// To3D takes an image pixel coordinate and returns the 3D coordinate in the world.
func (i *ImageWithDepth) To3D(p image.Point) (r3.Vector, error) {
	if i.camera == nil {
		return r3.Vector{}, errors.New("no camera on ImageWithDepth when called To3D")
	}
	if !p.In(i.Bounds()) {
		return r3.Vector{}, errors.Errorf("point (%d,%d) not within image bounds", p.X, p.Y)
	}
	d := i.Depth.Get(p)
	return i.camera.ImagePointTo3DPoint(p, d)
}

// Width returns the horizontal width of the image.
func (i *ImageWithDepth) Width() int {
	return i.Color.Width()
}

// Height returns the vertical height of the image.
func (i *ImageWithDepth) Height() int {
	return i.Color.Height()
}

// Rotate rotates the color and depth about the origin by the given angle clockwise.
func (i *ImageWithDepth) Rotate(amount int) *ImageWithDepth {
	return &ImageWithDepth{i.Color.Rotate(amount), i.Depth.Rotate(amount), i.aligned, i.camera}
}

// Warp adapts the image to a new size.
func (i *ImageWithDepth) Warp(src, dst []image.Point, newSize image.Point) *ImageWithDepth {
	m2 := GetPerspectiveTransform(src, dst)

	img := WarpImage(i.Color, m2, newSize)

	var warpedDepth *DepthMap
	if i.Depth != nil && i.Depth.Width() > 0 {
		warpedDepth = i.Depth.Warp(m2, newSize)
	}

	return &ImageWithDepth{ConvertImage(img), warpedDepth, i.aligned, i.camera}
}

// CropToDepthData TODO.
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

	return i.Warp(
		[]image.Point{{minX, minY}, {maxX, minY}, {maxX, maxY}, {minX, maxY}},
		[]image.Point{{0, 0}, {width, 0}, {width, height}, {0, height}},
		image.Point{width, height},
	), nil
}

// Overlay TODO.
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

// WriteTo writes both the color and depth data to the given file.
func (i *ImageWithDepth) WriteTo(fn string) error {
	return WriteBothToFile(i, fn)
}

// NewImageWithDepthFromImages returns a new image from the given two color and image files.
func NewImageWithDepthFromImages(colorFN, depthFN string, isAligned bool) (*ImageWithDepth, error) {
	img, err := NewImageFromFile(colorFN)
	if err != nil {
		return nil, errors.Wrapf(err, "cannot read color file (%s)", colorFN)
	}

	dm, err := NewDepthMapFromImageFile(depthFN)
	if err != nil {
		return nil, errors.Wrapf(err, "cannot read depth file (%s)", depthFN)
	}

	return &ImageWithDepth{img, dm, isAligned, nil}, nil
}

// NewImageWithDepth returns a new image from the given color image and depth data files.
func NewImageWithDepth(colorFN, depthFN string, isAligned bool) (*ImageWithDepth, error) {
	img, err := NewImageFromFile(colorFN)
	if err != nil {
		return nil, errors.Wrapf(err, "cannot read color file (%s)", colorFN)
	}

	dm, err := ParseDepthMap(depthFN)
	if err != nil {
		return nil, errors.Wrapf(err, "cannot read depth file (%s)", depthFN)
	}

	if isAligned {
		if img.Width() != dm.Width() || img.Height() != dm.Height() {
			return nil, errors.Errorf("color and depth size doesn't match %d,%d vs %d,%d",
				img.Width(), img.Height(), dm.Width(), dm.Height())
		}
	}

	return &ImageWithDepth{img, dm, isAligned, nil}, nil
}

func imageToDepthMap(img image.Image) *DepthMap {
	bounds := img.Bounds()

	width, height := bounds.Dx(), bounds.Dy()
	dm := NewEmptyDepthMap(width, height)

	grayImg, ok := img.(*image.Gray16)
	if !ok {
		panic(rdkutils.NewUnexpectedTypeError(grayImg, img))
	}
	for x := 0; x < width; x++ {
		for y := 0; y < height; y++ {
			i := grayImg.PixOffset(x, y)
			z := uint16(grayImg.Pix[i+0])<<8 | uint16(grayImg.Pix[i+1])
			dm.Set(x, y, Depth(z))
		}
	}

	return dm
}

// ConvertToImageWithDepth attempts to convert a go image into an image
// with depth, if it contains any.
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

// RawBytesWrite writes out the internal representation of the color
// and depth into the given buffer.
func (i *ImageWithDepth) RawBytesWrite(buf *bytes.Buffer) error {
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

// ImageWithDepthFromRawBytes returns a new image interpreted from the internal representation
// of color and depth.
func ImageWithDepthFromRawBytes(width, height int, b []byte) (*ImageWithDepth, error) {
	iwd := &ImageWithDepth{}

	// depth
	iwd.Depth = NewEmptyDepthMap(width, height)
	dst := utils.RawBytesFromSlice(iwd.Depth.data)
	read := copy(dst, b)
	if read != width*height*2 {
		return nil, errors.Errorf("invalid copy of depth data read: %d x: %d y: %d", read, width, height)
	}
	b = b[read:]

	iwd.Color = NewImage(width, height)
	dst = utils.RawBytesFromSlice(iwd.Color.data)
	read = copy(dst, b)
	if read != width*height*8 {
		return nil, errors.Errorf("invalid copy of color data read: %d x: %d y: %d", read, width, height)
	}
	b = b[read:]

	iwd.aligned = b[0] == 0x1

	return iwd, nil
}
