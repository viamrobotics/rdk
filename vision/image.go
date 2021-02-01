package vision

import (
	"compress/gzip"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"io/ioutil"
	"math"
	"os"
	"strings"

	"github.com/echolabsinc/robotcore/rcutil"

	"github.com/fogleman/gg"
)

type Image struct {
	img *image.RGBA

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

		img, _, err := readNextImageDepthPairFromBoth(allData)
		if err != nil {
			return Image{}, err
		}

		return NewImage(img), nil
	}

	f, err := os.Open(fn)
	if err != nil {
		return Image{}, err
	}
	img, _, err := image.Decode(f)
	if err != nil {
		return Image{}, err
	}
	return NewImage(img), nil
}

func NewImage(img image.Image) Image {
	bounds := img.Bounds()
	rgba := image.NewRGBA(bounds)
	draw.Draw(rgba, bounds, img, bounds.Min, draw.Src)
	return Image{img: rgba, height: bounds.Max.Y, width: bounds.Max.X}
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

	r, g, b, a := i.img.At(col, row).RGBA()
	return color.RGBA{uint8(r), uint8(g), uint8(b), uint8(a)}
}

func (i *Image) SetHSV(p image.Point, c HSV) {
	i.SetColor(p, c.ToColor())
}

func (i *Image) SetColor(p image.Point, c Color) {
	i.SetColorRowCol(p.Y, p.X, c)
}

func (i *Image) SetColorRowCol(row, col int, c Color) {
	i.img.Set(col, row, c)
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

// does not return a copy
func (i *Image) Image() image.Image {
	return i.img
}

// TODO(erh): move this to a better file
func PointDistance(a, b image.Point) float64 {
	x := rcutil.SquareInt(b.X - a.X)
	x += rcutil.SquareInt(b.Y - a.Y)
	return math.Sqrt(float64(x))
}

func (i *Image) WriteTo(fn string) error {
	f, err := os.Create(fn)
	if err != nil {
		return err
	}
	defer f.Close()
	return png.Encode(f, i.img)
}

func (i *Image) Circle(center image.Point, radius int, c Color) {
	dc := gg.NewContextForRGBA(i.img) // no copy
	dc.DrawCircle(float64(center.X), float64(center.Y), 1)
	dc.SetColor(c.C)
	dc.Fill()
}
