package vision

import (
	"fmt"
	"image"
	"image/color"
	"math"

	"gocv.io/x/gocv"

	"github.com/echolabsinc/robotcore/rcutil"
)

type Image struct {
	mat  gocv.Mat
	data []uint8

	width  int
	height int
}

func NewImageFromFile(fn string) (Image, error) {
	return NewImage(gocv.IMRead(fn, gocv.IMReadUnchanged))
}

func NewImage(mat gocv.Mat) (Image, error) {

	switch mat.Type() {
	case gocv.MatTypeCV8UC3:
		//good
	case gocv.MatTypeCV8UC4:
		gocv.CvtColor(mat, &mat, gocv.ColorBGRAToBGR)
	default:
		return Image{}, fmt.Errorf("bad mat type %v", mat.Type())
	}

	i := Image{mat, mat.DataPtrUint8(), mat.Cols(), mat.Rows()}

	if len(i.data) != (3 * i.width * i.height) {
		return Image{}, fmt.Errorf("bad length/size. len: %d width: %d height: %d", len(i.data), i.width, i.height)
	}

	return i, nil
}

func (i *Image) Close() {
	i.mat.Close()
}

func (i *Image) Rows() int {
	return i.height
}

func (i *Image) Cols() int {
	return i.width
}

func (i *Image) Height() int {
	return i.height
}

func (i *Image) Width() int {
	return i.width
}

func (i *Image) ColorHSV(p image.Point) HSV {
	return ConvertToHSV(i.ColorRowCol(p.Y, p.X))
}

func (i *Image) Color(p image.Point) color.RGBA {
	return i.ColorRowCol(p.Y, p.X)
}

func (i *Image) ColorXY(x, y int) color.RGBA {
	return i.ColorRowCol(y, x)
}

func (i *Image) ColorRowCol(row, col int) color.RGBA {

	if row < 0 || col < 0 || row >= i.height || col >= i.width {
		panic(fmt.Errorf("bad row or col want: %d %d width: %d height: %d", row, col, i.width, i.height))
	}

	base := 3 * (row*i.width + col)
	c := color.RGBA{}
	c.R = i.data[base+2]
	c.G = i.data[base+1]
	c.B = i.data[base+0]
	c.A = 1
	return c
}

func (i *Image) AverageColor(p image.Point) color.RGBA {
	return i.AverageColorXY(p.X, p.Y)
}

func (i *Image) AverageColorXY(x, y int) color.RGBA {
	b := 0
	g := 0
	r := 0

	num := 0

	for X := x - 1; X < x+1; X++ {
		for Y := y - 1; Y < y+1; Y++ {
			data := i.ColorRowCol(Y, X)
			b += int(data.B)
			g += int(data.G)
			r += int(data.R)
			num++
		}
	}

	done := color.RGBA{uint8(r / num), uint8(g / num), uint8(b / num), 1}
	return done

}

func (i *Image) MatUnsafe() gocv.Mat {
	return i.mat
}

// TODO(erh): move this to a better file
func PointDistance(a, b image.Point) float64 {
	x := rcutil.SquareInt(b.X - a.X)
	x += rcutil.SquareInt(b.Y - a.Y)
	return math.Sqrt(float64(x))
}
