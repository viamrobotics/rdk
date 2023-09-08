//go:build cgo
package rimage

import (
	"context"
	"image"
	"image/color"
	"image/png"
	"io"
	"math"
	"strconv"

	"github.com/pkg/errors"
	"gonum.org/v1/gonum/mat"

	"go.viam.com/rdk/utils"
)

// Depth is the depth in mm.
type Depth uint16

// MaxDepth is the maximum allowed depth.
const MaxDepth = Depth(math.MaxUint16)

// DepthMap fulfills the image.Image interface and represents the depth information of the scene in mm.
type DepthMap struct {
	width  int
	height int

	data []Depth
}

// NewEmptyDepthMap returns an unset depth map with the given dimensions.
func NewEmptyDepthMap(width, height int) *DepthMap {
	dm := &DepthMap{
		width:  width,
		height: height,
		data:   make([]Depth, width*height),
	}

	return dm
}

// Clone makes a copy of the depth map.
func (dm *DepthMap) Clone() *DepthMap {
	ddm := NewEmptyDepthMap(dm.Width(), dm.Height())
	copy(ddm.data, dm.data)
	return ddm
}

func (dm *DepthMap) kxy(x, y int) int {
	return (y * dm.width) + x
}

// Width returns the width of the depth map.
func (dm *DepthMap) Width() int {
	return dm.width
}

// Height returns the height of the depth map.
func (dm *DepthMap) Height() int {
	return dm.height
}

// Data returns the data from the depth map.
func (dm *DepthMap) Data() []Depth {
	return dm.data
}

// Bounds returns the rectangle dimensions of the image.
func (dm *DepthMap) Bounds() image.Rectangle {
	return image.Rect(0, 0, dm.width, dm.height)
}

// Get returns the depth at a given image.Point.
func (dm *DepthMap) Get(p image.Point) Depth {
	return dm.data[dm.kxy(p.X, p.Y)]
}

// GetDepth returns the depth at a given (x,y) coordinate.
func (dm *DepthMap) GetDepth(x, y int) Depth {
	return dm.data[dm.kxy(x, y)]
}

// Set sets the depth at a given (x,y) coordinate.
func (dm *DepthMap) Set(x, y int, val Depth) {
	dm.data[dm.kxy(x, y)] = val
}

// Contains returns whether or not a point is within bounds of the depth map.
func (dm *DepthMap) Contains(x, y int) bool {
	return x >= 0 && y >= 0 && x < dm.width && y < dm.height
}

// At returns the depth value as a color.Color so DepthMap can implement image.Image.
func (dm *DepthMap) At(x, y int) color.Color {
	return color.Gray16{uint16(dm.GetDepth(x, y))}
}

// ColorModel for DepthMap so that it implements image.Image.
func (dm *DepthMap) ColorModel() color.Model { return color.Gray16Model }

// SubImage returns a cropped image of the original DepthMap from the given rectangle.
func (dm *DepthMap) SubImage(rect image.Rectangle) *DepthMap {
	r := rect.Intersect(dm.Bounds()).Sub(dm.Bounds().Min)
	if r.Empty() {
		return NewEmptyDepthMap(0, 0)
	}
	newData := make([]Depth, 0, r.Dx()*r.Dy())
	for y := r.Min.Y; y < r.Max.Y; y++ {
		begin, end := (y*dm.width)+r.Min.X, (y*dm.width)+r.Max.X
		newData = append(newData, dm.data[begin:end]...)
	}
	return &DepthMap{width: r.Dx(), height: r.Dy(), data: newData}
}

// ConvertImageToDepthMap takes an image and figures out if it's already a DepthMap
// or if it can be converted into one.
func ConvertImageToDepthMap(ctx context.Context, img image.Image) (*DepthMap, error) {
	switch ii := img.(type) {
	case *LazyEncodedImage:
		lazyImg, _ := img.(*LazyEncodedImage)
		decodedImg, err := DecodeImage(ctx, lazyImg.RawData(), lazyImg.MIMEType())
		if err != nil {
			return nil, err
		}
		return ConvertImageToDepthMap(ctx, decodedImg)
	case *DepthMap:
		return ii, nil
	case *imageWithDepth:
		return ii.Depth, nil
	case *image.Gray16:
		return gray16ToDepthMap(ii), nil
	default:
		return nil, errors.Errorf("don't know how to make DepthMap from %T", img)
	}
}

// gray16ToDepthMap creates a DepthMap from an image.Gray16.
func gray16ToDepthMap(img *image.Gray16) *DepthMap {
	bounds := img.Bounds()
	width, height := bounds.Dx(), bounds.Dy()
	dm := NewEmptyDepthMap(width, height)
	for x := 0; x < width; x++ {
		for y := 0; y < height; y++ {
			i := img.PixOffset(x, y)
			z := uint16(img.Pix[i+0])<<8 | uint16(img.Pix[i+1])
			dm.Set(x, y, Depth(z))
		}
	}
	return dm
}

// ConvertImageToGray16 takes an image and figures out if it's already an image.Gray16
// or if it can be converted into one.
func ConvertImageToGray16(img image.Image) (*image.Gray16, error) {
	switch ii := img.(type) {
	case *DepthMap:
		return ii.ToGray16Picture(), nil
	case *imageWithDepth:
		return ii.Depth.ToGray16Picture(), nil
	case *image.Gray16:
		return ii, nil
	default:
		return nil, errors.Errorf("don't know how to make image.Gray16 from %T", img)
	}
}

// ToGray16Picture converts this depth map into a grayscale image of the same dimensions.
func (dm *DepthMap) ToGray16Picture() *image.Gray16 {
	grayScale := image.NewGray16(image.Rect(0, 0, dm.Width(), dm.Height()))

	for x := 0; x < dm.Width(); x++ {
		for y := 0; y < dm.Height(); y++ {
			val := dm.GetDepth(x, y)
			grayColor := color.Gray16{uint16(val)}
			grayScale.Set(x, y, grayColor)
		}
	}

	return grayScale
}

// WriteToBuf writes the depth map to a writer as 16bit grayscale png.
func (dm *DepthMap) WriteToBuf(out io.Writer) error {
	img := dm.ToGray16Picture()
	return png.Encode(out, img)
}

// MinMax returns the minimum and maximum depth values within the depth map.
func (dm *DepthMap) MinMax() (Depth, Depth) {
	min := MaxDepth
	max := Depth(0)

	for x := 0; x < dm.Width(); x++ {
		for y := 0; y < dm.Height(); y++ {
			z := dm.GetDepth(x, y)
			if z == 0 {
				continue
			}
			if z < min {
				min = z
			}
			if z > max {
				max = z
			}
		}
	}

	return min, max
}

// ToPrettyPicture converts the depth map into a colorful image to make it easier to see the depth gradients.
// The colorful picture will have no useful depth information, though.
func (dm *DepthMap) ToPrettyPicture(hardMin, hardMax Depth) *Image {
	min, max := dm.MinMax()

	if hardMin > 0 && min < hardMin {
		min = hardMin
	}
	if hardMax > 0 && max > hardMax {
		max = hardMax
	}

	img := NewImage(dm.Width(), dm.Height())

	span := float64(max) - float64(min)

	for x := 0; x < dm.Width(); x++ {
		for y := 0; y < dm.Height(); y++ {
			p := image.Point{x, y}
			z := dm.Get(p)
			if z == 0 {
				continue
			}

			if z < min {
				z = min
			}
			if z > max {
				z = max
			}

			ratio := float64(z-min) / span

			hue := 30 + (200.0 * ratio)
			img.SetXY(x, y, NewColorFromHSV(hue, 1.0, 1.0))
		}
	}

	return img
}

// Rotate rotates a copy of this depth map clockwise by the given amount.
func (dm *DepthMap) Rotate(amount int) *DepthMap {
	if amount == 0 {
		return dm
	}

	if amount == 180 {
		return dm.Rotate180()
	}

	if amount == 90 {
		return dm.Rotate90(true)
	}

	if amount == -90 || amount == 270 {
		return dm.Rotate90(false)
	}

	// made this a panic
	panic("vision.DepthMap can only rotate 90, -90, or 180 degrees, not " + strconv.Itoa(amount))
}

// Rotate90 rotates a copy of this depth map either by 90 degrees clockwise or counterclockwise.
func (dm *DepthMap) Rotate90(clockwise bool) *DepthMap {
	newWidth := dm.height
	newHeight := dm.width

	dm2 := &DepthMap{
		width:  newWidth,
		height: newHeight,
		data:   make([]Depth, newWidth*newHeight),
	}

	newCol, newRow := 0, 0
	if clockwise {
		for oldRow := dm.height - 1; oldRow >= 0; oldRow-- {
			newRow = 0
			for oldCol := 0; oldCol < dm.width; oldCol++ {
				val := dm.GetDepth(oldCol, oldRow)
				dm2.Set(newCol, newRow, val)
				newRow++
			}
			newCol++
		}
	} else { // counter-clockwise
		for oldCol := dm.width - 1; oldCol >= 0; oldCol-- {
			newCol = 0
			for oldRow := 0; oldRow < dm.height; oldRow++ {
				val := dm.GetDepth(oldCol, oldRow)
				dm2.Set(newCol, newRow, val)
				newCol++
			}
			newRow++
		}
	}
	return dm2
}

// Rotate180 rotates a copy of this depth map by 180 degrees.
func (dm *DepthMap) Rotate180() *DepthMap {
	dm2 := &DepthMap{
		width:  dm.width,
		height: dm.height,
		data:   make([]Depth, dm.width*dm.height),
	}

	k := 0 // optimization
	for y := 0; y < dm.height; y++ {
		for x := 0; x < dm.width; x++ {
			val := dm.GetDepth(dm.width-1-x, dm.height-1-y)
			dm2.data[k] = val
			// if k != dm2.kxy(x,y) { panic("oops") }
			k++
		}
	}
	return dm2
}

// AverageDepthAndStats returns average distance, average distance to avg.
// TODO(erh): should this be std. dev?
func (dm *DepthMap) AverageDepthAndStats(p image.Point, radius int) (float64, float64) {
	total := 0.0

	heights := []Depth{}

	for x := p.X - radius; x <= p.X+radius; x++ {
		if x < 0 || x >= dm.width {
			continue
		}
		for y := p.Y - radius; y <= p.Y+radius; y++ {
			if y < 0 || y >= dm.height {
				continue
			}

			h := dm.GetDepth(x, y)
			if h == 0 {
				continue
			}

			heights = append(heights, h)
			total += float64(h)
		}
	}

	if len(heights) == 0 {
		return 0.0, 0.0
	}

	avg := total / float64(len(heights))

	total = 0.0 // re-using for avg distance
	for _, h := range heights {
		d := math.Abs(float64(h) - avg)
		total += d
	}

	return avg, total / float64(len(heights))
}

// InterestingPixels TODO.
func (dm *DepthMap) InterestingPixels(t float64) *image.Gray {
	out := image.NewGray(dm.Bounds())

	for x := 0; x < dm.width; x += 3 {
		for y := 0; y < dm.height; y += 3 {
			_, avgDistance := dm.AverageDepthAndStats(image.Point{x + 1, y + 1}, 1)

			clr := color.Gray{0}
			if avgDistance > t {
				clr = color.Gray{255}
			}

			for a := 0; a < 3; a++ {
				for b := 0; b < 3; b++ {
					xx := x + a
					yy := y + b
					out.SetGray(xx, yy, clr)
				}
			}
		}
	}

	return out
}

type dmWarpConnector struct {
	In  *DepthMap
	Out *DepthMap
}

func (w *dmWarpConnector) Get(x, y int, buf []float64) bool {
	d := w.In.GetDepth(x, y)
	if d == 0 {
		return false
	}
	buf[0] = float64(d)
	return true
}

func (w *dmWarpConnector) Set(x, y int, data []float64) {
	w.Out.Set(x, y, Depth(data[0]))
}

func (w *dmWarpConnector) OutputDims() (int, int) {
	return w.Out.width, w.Out.height
}

func (w *dmWarpConnector) NumFields() int {
	return 1
}

// Warp returns a copy of this depth map warped by the given transformation matrix
// into a new size.
func (dm *DepthMap) Warp(m TransformationMatrix, newSize image.Point) *DepthMap {
	conn := &dmWarpConnector{dm, NewEmptyDepthMap(newSize.X, newSize.Y)}
	Warp(conn, m)
	return conn.Out
}

// ConvertDepthMapToLuminanceFloat converts this depth map into a grayscale image of the
// same dimensions.
func (dm *DepthMap) ConvertDepthMapToLuminanceFloat() *mat.Dense {
	out := mat.NewDense(dm.height, dm.width, nil)
	utils.ParallelForEachPixel(image.Point{dm.width, dm.height}, func(x, y int) {
		d := dm.GetDepth(x, y)
		out.Set(y, x, float64(d))
	})
	return out
}
