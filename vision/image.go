package vision

import (
	"compress/gzip"
	"fmt"
	"image"
	"image/color"
	"io/ioutil"
	"math"
	"os"
	"strings"

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
	if strings.HasSuffix(fn, ".both.gz") {

		f, err := os.Open(fn)
		if err != nil {
			return Image{}, err
		}
		defer f.Close()

		in, err := gzip.NewReader(f)
		if err != nil {
			return Image{}, err
		}

		defer in.Close()

		allData, err := ioutil.ReadAll(in)

		if err != nil {
			return Image{}, err
		}

		mat, _, err := readNextColorDepthPairFromBoth(allData)
		if err != nil {
			return Image{}, err
		}

		return NewImage(mat)
	}

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

	data, err := mat.DataPtrUint8()
	if err != nil {
		return Image{}, err
	}
	i := Image{mat, data, mat.Cols(), mat.Rows()}

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
	c.A = 255
	return c
}

func (i *Image) SetHSV(p image.Point, c HSV) {
	i.SetColor(p, c.ToColor())
}

func (i *Image) SetColor(p image.Point, c Color) {
	i.SetColorRowCol(p.Y, p.X, c)
}

func (i *Image) SetColorRowCol(row, col int, c Color) {
	base := 3 * (row*i.width + col)
	i.data[base+2] = c.C.R
	i.data[base+1] = c.C.G
	i.data[base+0] = c.C.B
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

func (i *Image) WriteTo(fn string) error {
	b := gocv.IMWrite(fn, i.mat)
	if b {
		return nil
	}
	return fmt.Errorf("couldn't write image to %s", fn)
}

func (i *Image) Circle(center image.Point, radius int, c Color) {
	gocv.Circle(&i.mat, center, radius, c.C, 1)
}
